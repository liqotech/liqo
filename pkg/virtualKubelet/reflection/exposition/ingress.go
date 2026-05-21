// Copyright 2019-2026 The Liqo Authors
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

package exposition

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	netv1clients "k8s.io/client-go/kubernetes/typed/networking/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	netv1listers "k8s.io/client-go/listers/networking/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	offloadingv1beta1clients "github.com/liqotech/liqo/pkg/client/clientset/versioned/typed/offloading/v1beta1"
	offloadingv1beta1listers "github.com/liqotech/liqo/pkg/client/listers/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ manager.NamespacedReflector = (*NamespacedIngressReflector)(nil)

const (
	// IngressReflectorName -> The name associated with the Ingress reflector.
	IngressReflectorName = "Ingress"
	liqoIngressClassName = "liqo"
)

// NamespacedIngressReflector manages the Ingress reflection for a given pair of local and remote namespaces.
type NamespacedIngressReflector struct {
	generic.NamespacedReflector

	localIngresses        netv1listers.IngressNamespaceLister
	remoteIngresses       netv1listers.IngressNamespaceLister
	remoteIngressesClient netv1clients.IngressInterface

	localShadowIngressStatuses     offloadingv1beta1listers.ShadowIngressStatusNamespaceLister
	localShadowIngressStatusClient offloadingv1beta1clients.ShadowIngressStatusInterface
	localNodes                     corev1listers.NodeLister

	enableIngress              bool
	remoteRealIngressClassName string
}

// NewIngressReflector returns a new IngressReflector instance.
func NewIngressReflector(reflectorConfig *offloadingv1beta1.ReflectorConfig,
	enableIngress bool, remoteRealIngressClassName string) manager.Reflector {
	return generic.NewReflector(IngressReflectorName, NewNamespacedIngressReflector(enableIngress, remoteRealIngressClassName),
		generic.WithoutFallback(), reflectorConfig.NumWorkers, reflectorConfig.Type, generic.ConcurrencyModeLeader)
}

// NewNamespacedIngressReflector returns a new NamespacedIngressReflector instance.
func NewNamespacedIngressReflector(enableIngress bool,
	remoteRealIngressClassName string) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return func(opts *options.NamespacedOpts) manager.NamespacedReflector {
		local := opts.LocalFactory.Networking().V1().Ingresses()
		remote := opts.RemoteFactory.Networking().V1().Ingresses()

		_, err := local.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)
		_, err = remote.Informer().AddEventHandler(opts.HandlerFactory(generic.NamespacedKeyer(opts.LocalNamespace)))
		utilruntime.Must(err)

		localShadow := opts.LocalLiqoFactory.Offloading().V1beta1().ShadowIngressStatuses()
		localNodes := opts.LocalFactory.Core().V1().Nodes()

		return &NamespacedIngressReflector{
			NamespacedReflector:            generic.NewNamespacedReflector(opts, IngressReflectorName),
			localIngresses:                 local.Lister().Ingresses(opts.LocalNamespace),
			remoteIngresses:                remote.Lister().Ingresses(opts.RemoteNamespace),
			remoteIngressesClient:          opts.RemoteClient.NetworkingV1().Ingresses(opts.RemoteNamespace),
			localShadowIngressStatuses:     localShadow.Lister().ShadowIngressStatuses(opts.LocalNamespace),
			localShadowIngressStatusClient: opts.LocalLiqoClient.OffloadingV1beta1().ShadowIngressStatuses(opts.LocalNamespace),
			localNodes:                     localNodes.Lister(),
			enableIngress:                  enableIngress,
			remoteRealIngressClassName:     remoteRealIngressClassName,
		}
	}
}

// Handle reconciles ingress objects.
func (nir *NamespacedIngressReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local Ingress %q (remote: %q)", nir.LocalRef(name), nir.RemoteRef(name))
	local, lerr := nir.localIngresses.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	remote, rerr := nir.remoteIngresses.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if rerr == nil && !forge.IsReflected(remote) {
		if lerr == nil { // Do not output the warning event in case the event was triggered by the remote object (i.e., the local one does not exists).
			klog.Infof("Skipping reflection of local Ingress %q as remote already exists and is not managed by us", nir.LocalRef(name))
			nir.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionAlreadyExistsMsg())
		}
		return nil
	}

	// Abort the reflection if the local object has the "skip-reflection" annotation.
	if !kerrors.IsNotFound(lerr) {
		skipReflection, err := nir.ShouldSkipReflection(local)
		if err != nil {
			klog.Errorf("Failed to check whether local Ingress %q should be reflected: %v", nir.LocalRef(name), err)
			return err
		}
		if skipReflection {
			if nir.GetReflectionType() == offloadingv1beta1.DenyList {
				klog.Infof("Skipping reflection of local Ingress %q as marked with the skip annotation", nir.LocalRef(name))
			} else { // AllowList
				klog.Infof("Skipping reflection of local Ingress %q as not marked with the allow annotation", nir.LocalRef(name))
			}
			nir.Event(local, corev1.EventTypeNormal, forge.EventReflectionDisabled, forge.EventObjectReflectionDisabledMsg(nir.GetReflectionType()))
			if kerrors.IsNotFound(rerr) { // The remote object does not already exist, hence no further action is required.
				return nil
			}

			// Otherwise, let pretend the local object does not exist, so that the remote one gets deleted.
			lerr = kerrors.NewNotFound(netv1.Resource("ingress"), local.GetName())
		}
	}

	tracer.Step("Performed the sanity checks")

	// The local ingress does no longer exist. Ensure it is also absent from the remote cluster.
	if kerrors.IsNotFound(lerr) {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(rerr) {
			klog.V(4).Infof("Deleting remote Ingress %q, since local %q does no longer exist", nir.RemoteRef(name), nir.LocalRef(name))
			if err := nir.DeleteRemote(ctx, nir.remoteIngressesClient, IngressReflectorName, name, remote.GetUID()); err != nil {
				return err
			}
		}

		// Delete the ShadowIngressStatus if it exists.
		shadowName := fmt.Sprintf("%s-%s", name, forge.RemoteCluster)
		if err := nir.localShadowIngressStatusClient.Delete(ctx, shadowName, metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			klog.Errorf("Failed to delete ShadowIngressStatus %q: %v", shadowName, err)
		}

		klog.V(4).Infof("Local Ingress %q and remote Ingress %q both vanished", nir.LocalRef(name), nir.RemoteRef(name))
		return nil
	}

	// Forge the mutation to be applied to the remote cluster.
	mutation := forge.RemoteIngress(local, nir.RemoteNamespace(), nir.enableIngress, nir.remoteRealIngressClassName, nir.ForgingOpts)
	tracer.Step("Remote mutation created")

	defer tracer.Step("Enforced the correctness of the remote object")
	if _, err := nir.remoteIngressesClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce remote Ingress %q (local: %q): %v", nir.RemoteRef(name), nir.LocalRef(name), err)
		nir.Event(local, corev1.EventTypeWarning, forge.EventFailedReflection, forge.EventFailedReflectionMsg(err))
		return err
	}

	klog.Infof("Remote Ingress %q successfully enforced (local: %q)", nir.RemoteRef(name), nir.LocalRef(name))
	nir.Event(local, corev1.EventTypeNormal, forge.EventSuccessfulReflection, forge.EventSuccessfulReflectionMsg())

	// Reconcile the ShadowIngressStatus for status aggregation.
	if err := nir.reconcileShadowIngressStatus(ctx, local, remote, name); err != nil {
		klog.Errorf("Failed to reconcile ShadowIngressStatus for Ingress %q: %v", nir.LocalRef(name), err)
	}

	return nil
}

// reconcileShadowIngressStatus creates, updates or deletes the ShadowIngressStatus for the given ingress.
func (nir *NamespacedIngressReflector) reconcileShadowIngressStatus(ctx context.Context,
	local, remote *netv1.Ingress, name string) error {
	// Check if the local ingress has the Liqo ingress class.
	hasLiqoClass := (local.Spec.IngressClassName != nil && *local.Spec.IngressClassName == liqoIngressClassName) ||
		local.Annotations["kubernetes.io/ingress.class"] == liqoIngressClassName

	shadowName := fmt.Sprintf("%s-%s", name, forge.RemoteCluster)

	if !hasLiqoClass {
		// If a ShadowIngressStatus exists, delete it (user may have changed class).
		if err := nir.localShadowIngressStatusClient.Delete(ctx, shadowName, metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ShadowIngressStatus %q: %w", shadowName, err)
		}
		return nil
	}

	if remote == nil || len(remote.Status.LoadBalancer.Ingress) == 0 {
		// Remote ingress does not exist or has no status. Delete the shadow if it exists.
		if err := nir.localShadowIngressStatusClient.Delete(ctx, shadowName, metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ShadowIngressStatus %q: %w", shadowName, err)
		}
		return nil
	}

	// Retrieve the local node to set the OwnerReference UID.
	node, err := nir.localNodes.Get(forge.LiqoNodeName)
	if err != nil {
		return fmt.Errorf("failed to get local node %q: %w", forge.LiqoNodeName, err)
	}

	shadow := &offloadingv1beta1.ShadowIngressStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shadowName,
			Namespace: nir.LocalNamespace(),
			Labels: map[string]string{
				forge.LiqoOriginClusterIDKey: string(forge.RemoteCluster),
				"liqo.io/ingress-name":       name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "Node",
					Name:               forge.LiqoNodeName,
					UID:                node.GetUID(),
					BlockOwnerDeletion: ptr.To(false),
				},
			},
		},
		Spec: offloadingv1beta1.ShadowIngressStatusSpec{
			IngressName:  name,
			ClusterID:    string(forge.RemoteCluster),
			LoadBalancer: *remote.Status.LoadBalancer.DeepCopy(),
		},
	}

	existing, err := nir.localShadowIngressStatuses.Get(shadowName)
	if kerrors.IsNotFound(err) {
		_, err = nir.localShadowIngressStatusClient.Create(ctx, shadow, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ShadowIngressStatus %q: %w", shadowName, err)
		}
		klog.V(4).Infof("Created ShadowIngressStatus %q", shadowName)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get ShadowIngressStatus %q: %w", shadowName, err)
	}

	// Verify the label matches our cluster ID (HA safety check).
	if existing.Labels[forge.LiqoOriginClusterIDKey] != string(forge.RemoteCluster) {
		klog.Warningf("ShadowIngressStatus %q exists but belongs to a different cluster. Skipping update.", shadowName)
		return nil
	}

	shadow.ResourceVersion = existing.ResourceVersion
	_, err = nir.localShadowIngressStatusClient.Update(ctx, shadow, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ShadowIngressStatus %q: %w", shadowName, err)
	}
	klog.V(4).Infof("Updated ShadowIngressStatus %q", shadowName)
	return nil
}

// List returns the list of ingress objects to be reflected.
func (nir *NamespacedIngressReflector) List() ([]interface{}, error) {
	return virtualkubelet.List[virtualkubelet.Lister[*netv1.Ingress], *netv1.Ingress](
		nir.localIngresses,
		nir.remoteIngresses,
	)
}

// Cleanup deletes all ShadowIngressStatus resources created by this reflector for the given namespace.
func (nir *NamespacedIngressReflector) Cleanup(ctx context.Context, _, _ string) error {
	selector := labels.SelectorFromSet(labels.Set{forge.LiqoOriginClusterIDKey: string(forge.RemoteCluster)})
	shadows, err := nir.localShadowIngressStatuses.List(selector)
	if err != nil {
		return fmt.Errorf("failed to list ShadowIngressStatuses for cleanup: %w", err)
	}
	for _, shadow := range shadows {
		if err := nir.localShadowIngressStatusClient.Delete(ctx, shadow.Name, metav1.DeleteOptions{}); err != nil && !kerrors.IsNotFound(err) {
			klog.Errorf("Failed to delete ShadowIngressStatus %q during cleanup: %v", shadow.Name, err)
		}
	}
	return nil
}
