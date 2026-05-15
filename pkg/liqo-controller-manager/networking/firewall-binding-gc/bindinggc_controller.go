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

package firewallbindinggc

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/firewall"
)

// DefaultBindingGCPeriod is the default period used to re-check whether a
// FirewallConfigurationBinding is still linked to a live target entity.
const DefaultBindingGCPeriod = 10 * time.Minute

// BindingGCReconciler garbage-collects FirewallConfigurationBinding resources that are no
// longer linked to their referenced target entity.
//
// A FirewallConfigurationBinding is created by the fabric or gateway binding creator
// controllers and carries a Spec.TargetRef that identifies the entity (e.g. a Pod or a
// Node) responsible for applying the referenced FirewallConfiguration and for removing
// the binding finalizer once the binding is deleted. If that entity disappears (e.g. a
// gateway pod is deleted without cleaning up, or a node is removed), the binding is never
// reconciled again and its finalizer is never removed, leaving it (and the referenced
// FirewallConfiguration) stuck forever.
//
// This controller periodically checks every binding. When it discovers a binding whose
// TargetRef no longer resolves to an existing object, it deletes the binding and forcefully
// removes the controller finalizer so that it (and any owning FirewallConfiguration) can be
// collected.
type BindingGCReconciler struct {
	client.Client

	// GCPeriod is the interval at which linked bindings are re-checked.
	GCPeriod time.Duration
}

// NewBindingGCReconciler returns a new BindingGCReconciler.
func NewBindingGCReconciler(cl client.Client, gcPeriod time.Duration) *BindingGCReconciler {
	if gcPeriod <= 0 {
		gcPeriod = DefaultBindingGCPeriod
	}
	return &BindingGCReconciler{
		Client:   cl,
		GCPeriod: gcPeriod,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile checks whether the given FirewallConfigurationBinding is still linked to a live
// target entity. If it is not, the binding is deleted and its controller finalizer is
// forcefully removed. Otherwise the binding is re-enqueued after GCPeriod for a later check.
func (r *BindingGCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	binding := &networkingv1beta1.FirewallConfigurationBinding{}
	if err := r.Get(ctx, req.NamespacedName, binding); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting firewallconfigurationbinding %s: %w", req.String(), err)
	}

	linked, err := r.isTargetLinked(ctx, binding.Spec.TargetRef)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("checking target link for firewallconfigurationbinding %s: %w", req.String(), err)
	}

	if linked {
		// The target entity is still alive: nothing to do now, re-check later.
		return ctrl.Result{RequeueAfter: r.GCPeriod}, nil
	}

	klog.Infof("FirewallConfigurationBinding %s is not linked to target %s: garbage collecting",
		req.String(), binding.Spec.TargetRef.String())

	return ctrl.Result{}, r.garbageCollect(ctx, binding)
}

// garbageCollect deletes the binding (if not already being deleted) and forcefully removes
// the controller finalizer so that the binding can be collected even if the target entity
// that normally handles cleanup no longer exists.
func (r *BindingGCReconciler) garbageCollect(ctx context.Context, binding *networkingv1beta1.FirewallConfigurationBinding) error {
	// Trigger deletion if it has not been requested yet.
	if binding.DeletionTimestamp.IsZero() {
		if err := r.Delete(ctx, binding); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("deleting firewallconfigurationbinding %s/%s: %w", binding.Namespace, binding.Name, err)
		}
		klog.Infof("Requested deletion of orphaned FirewallConfigurationBinding %s/%s", binding.Namespace, binding.Name)
	}

	// Forcefully remove the controller finalizer. Its owner (the gateway/fabric entity)
	// is gone and will never remove it, so the binding would otherwise be stuck forever.
	if ctrlutil.ContainsFinalizer(binding, firewall.FirewallConfigurationBindingControllerFinalizer) {
		original := binding.DeepCopy()
		ctrlutil.RemoveFinalizer(binding, firewall.FirewallConfigurationBindingControllerFinalizer)
		if err := r.Patch(ctx, binding, client.MergeFrom(original)); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("removing finalizer from firewallconfigurationbinding %s/%s: %w", binding.Namespace, binding.Name, err)
		}
		klog.Infof("Forcefully removed finalizer from orphaned FirewallConfigurationBinding %s/%s", binding.Namespace, binding.Name)
	}

	return nil
}

// isTargetLinked returns true if the given target reference resolves to an existing object.
// It is fully generic: it builds an unstructured object from the ref's GroupVersionKind and
// performs a single GET using the ref's namespace and name. Cluster-scoped targets must leave
// namespace empty.
func (r *BindingGCReconciler) isTargetLinked(ctx context.Context, ref networkingv1beta1.TargetReference) (bool, error) {
	// A binding without a target is not linked to anything.
	if ref.Name == "" {
		return false, nil
	}

	obj, err := newUnstructuredForTargetRef(ref)
	if err != nil {
		return false, err
	}

	if err := r.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting target %s/%s %s/%s: %w", ref.APIVersion, ref.Kind, ref.Namespace, ref.Name, err)
	}

	return true, nil
}

// newUnstructuredForTargetRef builds an empty unstructured object carrying the GVK described
// by the target reference.
func newUnstructuredForTargetRef(ref networkingv1beta1.TargetReference) (*unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("parsing apiVersion %q: %w", ref.APIVersion, err)
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    ref.Kind,
	})
	return obj, nil
}

// SetupWithManager registers the BindingGCReconciler with the manager. The controller
// garbage-collects orphaned bindings driven both by the periodic re-check (RequeueAfter)
// and by immediate watch events on Nodes and gateway Pods, so that deleting a fabric Node
// or a gateway Pod triggers prompt cleanup of the related FirewallConfigurationBindings.
func (r *BindingGCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	deleteOnly := predicate.Funcs{
		CreateFunc:  func(_ event.CreateEvent) bool { return false },
		UpdateFunc:  func(_ event.UpdateEvent) bool { return false },
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlFirewallConfigurationBindingGC).
		For(&networkingv1beta1.FirewallConfigurationBinding{}).
		Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(r.bindingEnqueuerForTarget), builder.WithPredicates(deleteOnly)).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(r.bindingEnqueuerForTarget), builder.WithPredicates(deleteOnly)).
		Complete(r)
}

// bindingEnqueuerForTarget maps a deleted Node or Pod to the FirewallConfigurationBindings
// whose Spec.TargetRef points to it.
func (r *BindingGCReconciler) bindingEnqueuerForTarget(ctx context.Context, obj client.Object) []ctrl.Request {
	gvk, err := r.GroupVersionKindFor(obj)
	if err != nil {
		klog.Warningf("Unable to determine GVK for object %s/%s: %v", obj.GetNamespace(), obj.GetName(), err)
		return nil
	}
	apiVersion, kind := gvk.ToAPIVersionAndKind()

	bindingList := &networkingv1beta1.FirewallConfigurationBindingList{}
	if err := r.List(ctx, bindingList); err != nil {
		klog.Errorf("Failed to list FirewallConfigurationBindings for target %s/%s: %v",
			obj.GetNamespace(), obj.GetName(), err)
		return nil
	}

	var requests []ctrl.Request
	for i := range bindingList.Items {
		ref := &bindingList.Items[i].Spec.TargetRef
		if ref.APIVersion == apiVersion && ref.Kind == kind &&
			ref.Namespace == obj.GetNamespace() && ref.Name == obj.GetName() {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: bindingList.Items[i].Namespace,
					Name:      bindingList.Items[i].Name,
				},
			})
		}
	}
	return requests
}
