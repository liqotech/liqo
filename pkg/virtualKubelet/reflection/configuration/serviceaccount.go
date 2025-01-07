// Copyright 2019-2025 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configuration

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/pod"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	// ServiceAccountReflectorName is the name associated with the ServiceAccount reflector.
	ServiceAccountReflectorName = "ServiceAccount"
)

// ServiceAccountReflector manages the reflection of tokens associated with ServiceAccounts.
type ServiceAccountReflector struct {
	manager.Reflector

	localPods corev1listers.PodLister
}

// NamespacedServiceAccountReflector manages the reflection of tokens associated with ServiceAccounts.
type NamespacedServiceAccountReflector struct {
	generic.NamespacedReflector

	localPods     corev1listers.PodNamespaceLister
	remoteSecrets corev1listers.SecretNamespaceLister

	localSAsClient      corev1clients.ServiceAccountInterface
	remoteSecretsClient corev1clients.SecretInterface

	podTokens sync.Map /* implicit signature: map[string]*forge.ServiceAccountPodTokens */
}

// FallbackServiceAccountReflector handles the "orphan" pods outside the managed namespaces.
// It ensures that the events for already existing pods are correctly emitted if the corresponding namespace gets offloaded afterwards.
type FallbackServiceAccountReflector struct {
	localPods corev1listers.PodLister
	ready     func() bool
}

// NewServiceAccountReflector builds a ServiceAccountReflector.
func NewServiceAccountReflector(enableSAReflection bool, reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	if !enableSAReflection {
		reflectorConfig.NumWorkers = 0
	}

	reflector := &ServiceAccountReflector{}
	genericReflector := generic.NewReflector(ServiceAccountReflectorName, reflector.NewNamespaced,
		reflector.NewFallback, reflectorConfig.NumWorkers, offloadingv1beta1.CustomLiqo, generic.ConcurrencyModeAll)
	reflector.Reflector = genericReflector
	return reflector
}

// RemoteSASecretNamespacedKeyer returns a keyer associated with the given namespace,
// which accounts for enqueuing only the secrets associated with a service account (enqueuing the owner pod name).
func RemoteSASecretNamespacedKeyer(namespace, nodename string) func(metadata metav1.Object) []types.NamespacedName {
	return func(metadata metav1.Object) []types.NamespacedName {
		if !forge.IsServiceAccountSecret(metadata) {
			return nil
		}

		label, ok := metadata.GetLabels()[forge.LiqoOriginClusterNodeName]
		if !ok || label != nodename {
			return []types.NamespacedName{}
		}

		// The label is certainly present, since it matched the selector.
		po := metadata.GetLabels()[forge.LiqoSASecretForPodNameKey]
		return []types.NamespacedName{{Namespace: namespace, Name: po}}
	}
}

// NewNamespaced returns a new NamespacedServiceAccountReflector instance.
func (sar *ServiceAccountReflector) NewNamespaced(opts *options.NamespacedOpts) manager.NamespacedReflector {
	remoteSecrets := opts.RemoteFactory.Core().V1().Secrets()

	// Regardless of the type of the event, we always enqueue the key corresponding to the pod.
	_, err := remoteSecrets.Informer().AddEventHandler(opts.HandlerFactory(RemoteSASecretNamespacedKeyer(opts.LocalNamespace, forge.LiqoNodeName)))
	utilruntime.Must(err)

	return &NamespacedServiceAccountReflector{
		NamespacedReflector: generic.NewNamespacedReflector(opts, ServiceAccountReflectorName),

		localPods:     sar.localPods.Pods(opts.LocalNamespace),
		remoteSecrets: remoteSecrets.Lister().Secrets(opts.RemoteNamespace),

		localSAsClient:      opts.LocalClient.CoreV1().ServiceAccounts(opts.LocalNamespace),
		remoteSecretsClient: opts.RemoteClient.CoreV1().Secrets(opts.RemoteNamespace),
	}
}

// NewFallback returns a new FallbackReflector instance.
func (sar *ServiceAccountReflector) NewFallback(opts *options.ReflectorOpts) manager.FallbackReflector {
	// Update events of the pod are ignored, since pod volumes are immutable.
	opts.LocalPodInformer.Informer().AddEventHandler(opts.HandlerFactory(generic.BasicKeyer(), options.EventFilterUpdate))

	return &FallbackServiceAccountReflector{
		localPods: opts.LocalPodInformer.Lister(),
		ready:     opts.Ready,
	}
}

// Start starts the reflector.
func (sar *ServiceAccountReflector) Start(ctx context.Context, opts *options.ReflectorOpts) {
	sar.localPods = opts.LocalPodInformer.Lister()
	sar.Reflector.Start(ctx, opts)
}

// Handle is responsible for reconciling the given object and ensuring it is correctly reflected.
func (nsar *NamespacedServiceAccountReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Retrieve the local object (only not found errors can occur).
	klog.V(4).Infof("Handling service account reflection for local pod %q", nsar.LocalRef(name))
	rname := forge.ServiceAccountSecretName(name)
	local, lerr := nsar.localPods.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := nsar.remoteSecrets.Get(rname)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if rerr == nil && (!forge.IsReflected(remote) || !forge.IsServiceAccountSecret(remote)) {
		if lerr == nil { // Do not output the warning event in case the event was triggered by the remote object (i.e., the local one does not exists).
			klog.Infof("Skipping reflection of the SA tokens secret for pod %q as remote already exists and is not managed by us", nsar.LocalRef(name))
			nsar.Event(local, corev1.EventTypeWarning, forge.EventFailedSATokensReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}
	tracer.Step("Performed the sanity checks")

	// Remove the secret if the pod does no longer exist.
	if kerrors.IsNotFound(lerr) {
		return nsar.enforceSecretAbsence(ctx, name, rname, remote)
	}

	// Retrieve the tokens required by the given pod.
	var tokens *forge.ServiceAccountPodTokens
	if loaded, ok := nsar.podTokens.Load(name); ok {
		tokens = loaded.(*forge.ServiceAccountPodTokens)
	} else {
		tokens = nsar.buildTokensInfo(local, remote)
		nsar.podTokens.Store(name, tokens)
	}
	tracer.Step("Retrieved the cached tokens")

	// There is a limited likelihood that the cached tokens refer to a different pod.
	// e.g., in case the pod is force deleted while the virtual kubelet is not running.
	// In this case, the cached tokens and the remote secret are deleted, and the pod is reenqueued for being handled again.
	if tokens.PodUID != local.GetUID() {
		// The error is ignored, since this pod is nonetheless reenqueued.
		klog.Warningf("Mismatching UID between local pod %q and cache: regenerating tokens...", nsar.LocalRef(name))
		_ = nsar.enforceSecretAbsence(ctx, name, rname, remote)
		return fmt.Errorf("mismatching UID detected for pod %q", klog.KObj(local))
	}

	if len(tokens.Tokens) == 0 {
		klog.Infof("Skipping reflection of SA tokens secret for local pod %q, as none is mounted", nsar.LocalRef(name))
		return nil
	}

	// Check whether any of the tokens needs to be refreshed.
	for _, token := range tokens.Tokens {
		if err := nsar.handleToken(ctx, local, token, tokens.ServiceAccountName); err != nil {
			nsar.Event(local, corev1.EventTypeWarning, forge.EventFailedSATokensReflection, forge.EventFailedReflectionMsg(err))
			return err
		}
	}

	mutation := forge.RemoteServiceAccountSecret(tokens, rname, nsar.RemoteNamespace(), local.Spec.NodeName)
	tracer.Step("Remote mutation created")

	defer tracer.Step("Enforced the correctness of the remote object")
	if _, err := nsar.remoteSecretsClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce remote SA tokens secret %q (local pod: %q): %v", nsar.RemoteRef(rname), nsar.LocalRef(name), err)
		nsar.Event(local, corev1.EventTypeWarning, forge.EventFailedSATokensReflection, forge.EventFailedReflectionMsg(err))
		return err
	}

	klog.Infof("Remote SA tokens secret %q successfully enforced (local pod: %q)", nsar.RemoteRef(rname), nsar.LocalRef(name))
	nsar.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulSATokensReflection, forge.EventSuccessfulReflectionMsg())

	// Reenqueue the event, to ensure tokens is refreshed before expiration.
	refreshAfter := utils.RandomJitter(time.Until(tokens.EarliestRefresh()), 30*time.Second)
	klog.V(4).Infof("Scheduling refresh of SA tokens for local pod %q in %v", nsar.LocalRef(name), refreshAfter)
	return generic.EnqueueAfter(refreshAfter)
}

func (nsar *NamespacedServiceAccountReflector) handleToken(ctx context.Context, local *corev1.Pod,
	token *forge.ServiceAccountPodToken, saname string) error {
	name := local.GetName()

	// Check whether the token needs to be refreshed (i.e., if more than 80% of its lifespan passed).
	if token.Token != "" && token.RefreshDue().After(time.Now()) {
		klog.V(4).Infof("Skipping refresh of SA token %q for local pod %q, as still valid", token.Key, nsar.LocalRef(name))
		return nil
	}

	defer trace.FromContext(ctx).Step(fmt.Sprintf("Refreshed SA token %q", token.Key))
	klog.V(4).Infof("Refreshing SA token %q for local pod %q (service account: %q)", token.Key, nsar.LocalRef(name), nsar.LocalRef(saname))

	response, err := nsar.localSAsClient.CreateToken(ctx, saname, token.TokenRequest(local), metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("Failed to refresh SA token %q for local pod %q (service account: %q): %v",
			token.Key, nsar.LocalRef(name), nsar.LocalRef(saname), err)
		return err
	}

	klog.Infof("SA token %q for local pod %q (service account: %q) successfully refreshed", token.Key, nsar.LocalRef(name), nsar.LocalRef(saname))
	token.Update(response.Status.Token, response.Status.ExpirationTimestamp.Time)
	return nil
}

func (nsar *NamespacedServiceAccountReflector) enforceSecretAbsence(ctx context.Context, name, rname string, remote *corev1.Secret) error {
	defer trace.FromContext(ctx).Step("Ensured the absence of the remote object")

	nsar.podTokens.Delete(name)
	if remote != nil {
		klog.V(4).Infof("Deleting remote SA tokens secret %q, since local pod %q does no longer exist", nsar.RemoteRef(rname), nsar.LocalRef(name))
		return nsar.DeleteRemote(ctx, nsar.remoteSecretsClient, SecretReflectorName, rname, remote.GetUID())
	}

	klog.V(4).Infof("Local pod %q and remote SA tokens secret %q both vanished", nsar.LocalRef(name), nsar.RemoteRef(rname))
	return nil
}

func (nsar *NamespacedServiceAccountReflector) buildTokensInfo(po *corev1.Pod, secret *corev1.Secret) *forge.ServiceAccountPodTokens {
	saName := pod.ServiceAccountName(po)
	expiration := forge.ServiceAccountTokenExpirationFromSecret(secret)
	tokens := forge.ServiceAccountPodTokens{
		PodName:            po.GetName(),
		PodUID:             forge.ServiceAccountPodUIDFromSecret(secret, po.GetUID()),
		ServiceAccountName: saName,
	}

	// Iterate over the volumes associated with the pod, looking for service account tokens.
	for i := range po.Spec.Volumes {
		projected := po.Spec.Volumes[i].Projected

		if projected != nil {
			for j := range projected.Sources {
				saToken := projected.Sources[j].ServiceAccountToken
				if saToken != nil {
					key := forge.ServiceAccountTokenKey(po.Spec.Volumes[i].Name, saToken.Path)
					token := tokens.AddToken(key, saToken.Audience,
						pointer.Int64PtrDerefOr(saToken.ExpirationSeconds, 3600 /* default is 1 hour */))
					token.Update(forge.ServiceAccountTokenFromSecret(secret, key), expiration)
				}
			}
		}
	}

	return &tokens
}

// List returns a list of all service account tokens managed by the reflector.
func (nsar *NamespacedServiceAccountReflector) List() ([]interface{}, error) {
	return virtualkubelet.List[virtualkubelet.Lister[*corev1.Secret], *corev1.Secret](
		nsar.remoteSecrets,
	)
}

// Handle operates as fallback to reconcile pod objects not managed by namespaced handlers.
func (fsar *FallbackServiceAccountReflector) Handle(ctx context.Context, key types.NamespacedName) error {
	// No operation needs to be performed here, as the fallback reflector is only used to reenqueue possible already
	// existing pods living in a namespace that has just been offloaded (since they would otherwise be missed).
	return nil
}

// Keys returns a set of keys to be enqueued for fallback processing for the given namespace pair.
func (fsar *FallbackServiceAccountReflector) Keys(local, _ string) []types.NamespacedName {
	pods, err := fsar.localPods.Pods(local).List(labels.Everything())
	utilruntime.Must(err)

	keys := make([]types.NamespacedName, 0, len(pods))
	keyer := generic.BasicKeyer()
	for _, pod := range pods {
		keys = append(keys, keyer(pod)...)
	}
	return keys
}

// Ready returns whether the FallbackReflector is completely initialized.
func (fsar *FallbackServiceAccountReflector) Ready() bool {
	return fsar.ready()
}

// List returns a list of all service account tokens managed by the reflector.
func (fsar *FallbackServiceAccountReflector) List() ([]interface{}, error) {
	return virtualkubelet.List[virtualkubelet.Lister[*corev1.Pod], *corev1.Pod](
		fsar.localPods,
	)
}
