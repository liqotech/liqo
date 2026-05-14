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

package internalfabriccontroller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// InternalFabricReconciler manage InternalFabric lifecycle.
type InternalFabricReconciler struct {
	client.Client
	Scheme                         *runtime.Scheme
	RouteConfigurationRulePriority int
}

// NewInternalFabricReconciler returns a new InternalFabricReconciler.
func NewInternalFabricReconciler(cl client.Client, s *runtime.Scheme, routeConfigurationRulePriority int) *InternalFabricReconciler {
	return &InternalFabricReconciler{
		Client:                         cl,
		Scheme:                         s,
		RouteConfigurationRulePriority: routeConfigurationRulePriority,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=genevetunnels,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=genevetunnels/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes/finalizers,verbs=update

// Reconcile manage InternalFabric lifecycle.
func (r *InternalFabricReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	internalFabric := &networkingv1beta1.InternalFabric{}
	if err := r.Get(ctx, req.NamespacedName, internalFabric); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("InternalFabric %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting InternalFabric: %w", err)
	}

	if !internalFabric.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(internalFabric, consts.InternalFabricGeneveTunnelFinalizer) {
			if err := deleteGeneveTunnels(ctx, r.Client, internalFabric); err != nil {
				return ctrl.Result{}, fmt.Errorf("deleting Geneve tunnels: %w", err)
			}
		}

		// Remove the geneve tunnel finalizer and the old deprecated one from previous versions.
		updated := controllerutil.RemoveFinalizer(internalFabric, consts.InternalFabricGeneveTunnelFinalizer)
		updated = controllerutil.RemoveFinalizer(internalFabric, "internalfabric-controller.liqo.io/finalizer") || updated
		if updated {
			if err := r.Update(ctx, internalFabric); err != nil {
				return ctrl.Result{}, fmt.Errorf("removing finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// route configuration

	if err := r.ensureRouteConfiguration(ctx, internalFabric); err != nil {
		return ctrl.Result{}, fmt.Errorf("ensuring route configuration: %w", err)
	}

	// geneve tunnels

	var internalNodeList networkingv1beta1.InternalNodeList
	if err := r.List(ctx, &internalNodeList); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing InternalNodes: %w", err)
	}

	if err := ensureGeneveTunnels(ctx, r.Client, r.Scheme, internalFabric, &internalNodeList); err != nil {
		return ctrl.Result{}, fmt.Errorf("ensuring GeneveTunnels: %w", err)
	}

	if err := cleanupGeneveTunnels(ctx, r.Client, internalFabric, &internalNodeList); err != nil {
		return ctrl.Result{}, fmt.Errorf("cleaning up GeneveTunnels: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the InternalFabricReconciler to the manager.
func (r *InternalFabricReconciler) SetupWithManager(mgr ctrl.Manager) error {
	internalNodeEnqueuer := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			var requests []reconcile.Request

			var internalFabricList networkingv1beta1.InternalFabricList
			if err := r.List(ctx, &internalFabricList); err != nil {
				klog.Errorf("Unable to list InternalFabrics: %s", err)
				return nil
			}

			for i := range internalFabricList.Items {
				fabric := &internalFabricList.Items[i]

				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(fabric),
				})
			}

			return requests
		},
	)

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlInternalFabricCM).
		For(&networkingv1beta1.InternalFabric{}).
		Watches(&networkingv1beta1.InternalNode{}, internalNodeEnqueuer).
		Owns(&networkingv1beta1.RouteConfiguration{}).
		Owns(&networkingv1beta1.GeneveTunnel{}).
		Complete(r)
}
