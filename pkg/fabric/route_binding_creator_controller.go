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

//nolint:goconst,dupl // API versions are hard-coded to match the firewall binding-creator pattern.
package fabric

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/route"
	utilspredicates "github.com/liqotech/liqo/pkg/utils/predicates"
)

// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurationbindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurationbindings/finalizers,verbs=update

// RouteBindingCreatorReconciler reconciles RouteConfiguration resources targeted at
// this fabric node and creates the corresponding RouteConfigurationBinding resource.
type RouteBindingCreatorReconciler struct {
	route.BindingCreatorBase
	nodeName string
}

// NewFabricRouteBindingCreatorReconciler returns a new RouteBindingCreatorReconciler.
func NewFabricRouteBindingCreatorReconciler(cl client.Client, s *runtime.Scheme, nodeName string) *RouteBindingCreatorReconciler {
	return &RouteBindingCreatorReconciler{
		BindingCreatorBase: route.BindingCreatorBase{Client: cl, Scheme: s},
		nodeName:           nodeName,
	}
}

// Reconcile creates or updates the RouteConfigurationBinding resource for the
// current fabric node referenced by the given RouteConfiguration.
func (r *RouteBindingCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	routecfg := &networkingv1beta1.RouteConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, routecfg); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting routeconfiguration: %w", err)
	}

	if !routecfg.DeletionTimestamp.IsZero() {
		// RouteCfg is being deleted; GC will handle the owned bindings via ownerRef.
		return ctrl.Result{}, nil
	}

	klog.V(4).Infof("Reconciling fabric routeconfiguration binding resources for %s", req.String())

	targets := []networkingv1beta1.RouteBindingTargetReference{{
		APIVersion: route.TargetAPIVersionV1,
		Kind:       route.TargetKindNode,
		Name:       r.nodeName,
	}}

	if err := r.ReconcileBindings(ctx, routecfg, targets); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling fabric route bindings for %s: %w", req.String(), err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the FabricRouteBindingCreatorReconciler with the manager.
// The labelsSets argument defines which RouteConfiguration resources this fabric
// node is responsible for; only resources matching one of the sets are reconciled.
func (r *RouteBindingCreatorReconciler) SetupWithManager(mgr ctrl.Manager, labelsSets []labels.Set) error {
	filterByLabelsPredicate := utilspredicates.NewAnyLabelsSetPredicate(labelsSets)

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlFabricRouteConfigurationBindingCreator).
		For(&networkingv1beta1.RouteConfiguration{}, builder.WithPredicates(filterByLabelsPredicate)).
		Owns(&networkingv1beta1.RouteConfigurationBinding{}, builder.WithPredicates(
			route.ForgeRouteTargetRefPredicate(networkingv1beta1.RouteBindingTargetReference{
				APIVersion: route.TargetAPIVersionV1,
				Kind:       route.TargetKindNode,
				Name:       r.nodeName,
			}))).
		Complete(r)
}
