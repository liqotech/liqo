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

package gateway

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	firewallpkg "github.com/liqotech/liqo/pkg/firewall"
)

// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings/finalizers,verbs=update

// BindingCreatorReconciler reconciles FirewallConfiguration resources targeted at
// this gateway and creates the corresponding FirewallConfigurationBinding resource.
type BindingCreatorReconciler struct {
	firewallpkg.BindingCreatorBase
	gatewayName      string
	gatewayNamespace string
}

// NewGatewayBindingCreatorReconciler returns a new BindingCreatorReconciler.
// gatewayName and gatewayNamespace identify the gateway pod that will apply the bindings.
func NewGatewayBindingCreatorReconciler(cl client.Client, s *runtime.Scheme,
	gatewayName, gatewayNamespace string) *BindingCreatorReconciler {
	return &BindingCreatorReconciler{
		BindingCreatorBase: firewallpkg.BindingCreatorBase{Client: cl, Scheme: s},
		gatewayName:        gatewayName,
		gatewayNamespace:   gatewayNamespace,
	}
}

// Reconcile creates or updates the FirewallConfigurationBinding resource for the
// current gateway referenced by the given FirewallConfiguration.
func (r *BindingCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	fwcfg := &networkingv1beta1.FirewallConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, fwcfg); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting firewallconfiguration: %w", err)
	}

	if !fwcfg.DeletionTimestamp.IsZero() {
		// FWCfg is being deleted; GC will handle the owned bindings via ownerRef.
		return ctrl.Result{}, nil
	}

	klog.V(4).Infof("Reconciling gateway firewallconfiguration binding resources for %s", req.String())

	targets := []firewallpkg.BindingTarget{{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       r.gatewayName,
		Namespace:  r.gatewayNamespace,
	}}

	if err := r.ReconcileBindings(ctx, fwcfg, targets); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling gateway bindings for %s: %w", req.String(), err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the GatewayBindingCreatorReconciler with the manager.
// The labelsSets argument defines which FirewallConfiguration resources this gateway
// instance is responsible for; only resources matching one of the sets are reconciled.
func (r *BindingCreatorReconciler) SetupWithManager(mgr ctrl.Manager, labelsSets []labels.Set) error {
	filterByLabelsPredicate, err := forgeLabelsPredicate(labelsSets)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlGatewayFirewallConfigurationBindingCreator).
		For(&networkingv1beta1.FirewallConfiguration{}, builder.WithPredicates(filterByLabelsPredicate)).
		Owns(&networkingv1beta1.FirewallConfigurationBinding{}, builder.WithPredicates(
			firewallpkg.ForgeTargetRefPredicate("v1", "Pod", r.gatewayName, r.gatewayNamespace))).
		Complete(r)
}

// forgeLabelsPredicate returns a predicate that matches resources with any of the given label sets.
func forgeLabelsPredicate(labelsSets []labels.Set) (predicate.Predicate, error) {
	labelPredicates := make([]predicate.Predicate, len(labelsSets))
	for i := range labelsSets {
		p, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: labelsSets[i]})
		if err != nil {
			return nil, err
		}
		labelPredicates[i] = p
	}
	return predicate.Or(labelPredicates...), nil
}
