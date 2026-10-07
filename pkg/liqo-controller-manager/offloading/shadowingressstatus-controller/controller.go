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

package shadowingressstatusctrl

import (
	"context"
	"fmt"

	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

const (
	ingressNameLabelKey = "liqo.io/ingress-name"
)

// Reconciler reconciles Ingress objects based on ShadowIngressStatus resources.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=offloading.liqo.io,resources=shadowingressstatuses,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/status,verbs=get;patch;update

// Reconcile aggregates ShadowIngressStatus resources into the local Ingress status.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(4).Infof("reconcile ingress %q", req.NamespacedName)

	ingress := netv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if errors.IsNotFound(err) {
			// Ingress deleted: clean up orphan ShadowIngressStatuses.
			return r.cleanupOrphanShadows(ctx, req.NamespacedName)
		}
		return ctrl.Result{}, err
	}

	// List all ShadowIngressStatuses associated with this ingress.
	selector := labels.SelectorFromSet(labels.Set{ingressNameLabelKey: ingress.Name})
	var shadowList offloadingv1beta1.ShadowIngressStatusList
	if err := r.List(ctx, &shadowList,
		client.InNamespace(ingress.Namespace),
		client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list ShadowIngressStatuses: %w", err)
	}

	// Aggregate loadBalancer.ingress from all shadows.
	aggregated := make([]netv1.IngressLoadBalancerIngress, 0, len(shadowList.Items))
	for i := range shadowList.Items {
		aggregated = append(aggregated, shadowList.Items[i].Spec.LoadBalancer.Ingress...)
	}

	// Update the Ingress status.
	ingress.Status.LoadBalancer = netv1.IngressLoadBalancerStatus{Ingress: aggregated}
	if err := r.Client.Status().Update(ctx, &ingress); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update ingress status: %w", err)
	}

	klog.V(4).Infof("patched ingress %q status with %d loadbalancer entries", req.NamespacedName, len(aggregated))
	return ctrl.Result{}, nil
}

// cleanupOrphanShadows deletes ShadowIngressStatus resources whose associated Ingress has been removed.
func (r *Reconciler) cleanupOrphanShadows(ctx context.Context, ingressName types.NamespacedName) (ctrl.Result, error) {
	selector := labels.SelectorFromSet(labels.Set{ingressNameLabelKey: ingressName.Name})
	var shadowList offloadingv1beta1.ShadowIngressStatusList
	if err := r.List(ctx, &shadowList,
		client.InNamespace(ingressName.Namespace),
		client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return ctrl.Result{}, err
	}

	for i := range shadowList.Items {
		if err := r.Delete(ctx, &shadowList.Items[i]); err != nil && !errors.IsNotFound(err) {
			klog.Errorf("Failed to delete orphan ShadowIngressStatus %q: %v", shadowList.Items[i].Name, err)
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlShadowIngressStatus).
		For(&netv1.Ingress{}).
		Watches(&offloadingv1beta1.ShadowIngressStatus{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
				shadow, ok := obj.(*offloadingv1beta1.ShadowIngressStatus)
				if !ok {
					return nil
				}
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name:      shadow.Spec.IngressName,
						Namespace: shadow.Namespace,
					},
				}}
			}), builder.WithPredicates(predicate.Funcs{
				CreateFunc:  func(_ event.CreateEvent) bool { return true },
				UpdateFunc:  func(_ event.UpdateEvent) bool { return true },
				DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
				GenericFunc: func(_ event.GenericEvent) bool { return false },
			})).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}
