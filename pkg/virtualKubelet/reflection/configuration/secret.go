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

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	// SecretReflectorName is the name associated with the Secret reflector.
	SecretReflectorName = "Secret"
)

// NamespacedSecretReflector manages the Secret reflection.
type NamespacedSecretReflector struct {
	generic.NamespacedReflector

	localSecrets        corev1listers.SecretNamespaceLister
	remoteSecrets       corev1listers.SecretNamespaceLister
	remoteSecretsClient corev1clients.SecretInterface

	enableSAReflection bool
}

// NewSecretReflector builds a SecretReflector.
func NewSecretReflector(enableSAReflection bool, reflectorConfig *offloadingv1beta1.ReflectorConfig) manager.Reflector {
	return generic.NewReflector(SecretReflectorName, NewNamespacedSecretReflector(enableSAReflection),
		generic.WithoutFallback(), reflectorConfig.NumWorkers, reflectorConfig.Type, generic.ConcurrencyModeLeader)
}

// NewNamespacedSecretReflector returns a function generating NamespacedSecretReflector instances.
func NewNamespacedSecretReflector(enableSAReflection bool) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		local := opts.LocalFactory.Core().V1().Secrets()
		remote := opts.RemoteFactory.Core().V1().Secrets()

		// Using opts.LocalNamespace for both event handlers so that the object will be put in the same workqueue
		// no matter the cluster, hence it will be processed by the handle function in the same way.
		local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		remote.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))

		return &NamespacedSecretReflector{
			NamespacedReflector: generic.NewNamespacedReflector(opts, SecretReflectorName),
			localSecrets:        local.Lister().Secrets(opts.LocalNamespace),
			remoteSecrets:       remote.Lister().Secrets(opts.RemoteNamespace),
			remoteSecretsClient: opts.RemoteClient.CoreV1().Secrets(opts.RemoteNamespace),
			enableSAReflection:  enableSAReflection,
		}
	}
}

// Handle is responsible for reconciling the given object and ensuring it is correctly reflected.
func (nsr *NamespacedSecretReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local Secret %q (remote: %q)", nsr.LocalRef(name), nsr.RemoteRef(name))

	local, lerr := nsr.localSecrets.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := nsr.remoteSecrets.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the local object is a secret of type "kubernetes.io/service-account-token", and service account reflection is disabled.
	if !nsr.enableSAReflection && lerr == nil && local.Type == corev1.SecretTypeServiceAccountToken {
		klog.Infof("Skipping reflection of local Secret %q because of type %s", nsr.LocalRef(name), corev1.SecretTypeServiceAccountToken)
		nsr.Event(local, corev1.EventTypeNormal, forge.EventReflectionDisabled, forge.EventSAReflectionDisabledMsg())
		return nil
	}

	// Skip secrets containing service account tokens, as managed by the dedicated reflector.
	if rerr == nil && forge.IsServiceAccountSecret(remote) {
		klog.Infof("Skipping reflection of remote Secret %q as containing service account tokens", nsr.LocalRef(name))
		return nil
	}

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if rerr == nil && !forge.IsReflected(remote) {
		if lerr == nil { // Do not output the warning event in case the event was triggered by the remote object (i.e., the local one does not exists).
			klog.Infof("Skipping reflection of local Secret %q as remote already exists and is not managed by us", nsr.LocalRef(name))
			nsr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}

	// Abort the reflection if the local object has the "skip-reflection" annotation.
	if !kerrors.IsNotFound(lerr) {
		skipReflection, err := nsr.ShouldSkipReflection(local)
		if err != nil {
			klog.Errorf("Failed to check whether local Secret %q should be reflected: %v", nsr.LocalRef(name), err)
			return err
		}
		if skipReflection {
			if nsr.GetReflectionType() == offloadingv1beta1.DenyList {
				klog.Infof("Skipping reflection of local Secret %q as marked with the skip annotation", nsr.LocalRef(name))
			} else { // AllowList
				klog.Infof("Skipping reflection of local Secret %q as not marked with the allow annotation", nsr.LocalRef(name))
			}
			nsr.Event(local, corev1.EventTypeNormal, forge.EventReflectionDisabled, forge.EventObjectReflectionDisabledMsg(nsr.GetReflectionType()))
			if kerrors.IsNotFound(rerr) { // The remote object does not already exist, hence no further action is required.
				return nil
			}

			// Otherwise, let pretend the local object does not exist, so that the remote one gets deleted.
			lerr = kerrors.NewNotFound(corev1.Resource("secret"), local.GetName())
		}
	}

	tracer.Step("Performed the sanity checks")

	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote Secret %q, since local %q does no longer exist", nsr.RemoteRef(name), nsr.LocalRef(name))
			return nsr.DeleteRemote(ctx, nsr.remoteSecretsClient, SecretReflectorName, name, remote.GetUID())
		}

		klog.V(4).Infof("Local Secret %q and remote Secret %q both vanished", nsr.LocalRef(name), nsr.RemoteRef(name))
		return nil
	}

	// Forge the mutation to be applied to the remote cluster.
	mutation := forge.RemoteSecret(local, nsr.RemoteNamespace(), nsr.ForgingOpts)
	tracer.Step("Remote mutation created")

	defer tracer.Step("Enforced the correctness of the remote object")
	if _, err := nsr.remoteSecretsClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce remote Secret %q (local: %q): %v", nsr.RemoteRef(name), nsr.LocalRef(name), err)
		nsr.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return err
	}

	klog.Infof("Remote Secret %q successfully enforced (local: %q)", nsr.RemoteRef(name), nsr.LocalRef(name))
	nsr.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())

	return nil
}

// List returns the list of objects to be reflected.
func (nsr *NamespacedSecretReflector) List() ([]interface{}, error) {
	return virtualkubelet.List[virtualkubelet.Lister[*corev1.Secret], *corev1.Secret](
		nsr.localSecrets,
		nsr.remoteSecrets,
	)
}
