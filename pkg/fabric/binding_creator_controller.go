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
	firewallpkg "github.com/liqotech/liqo/pkg/firewall"
	utilspredicates "github.com/liqotech/liqo/pkg/utils/predicates"
)

// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings/finalizers,verbs=update

// BindingCreatorReconciler reconciles FirewallConfiguration resources targeted at
// this fabric node and creates the corresponding FirewallConfigurationBinding resource.
type BindingCreatorReconciler struct {
	firewallpkg.BindingCreatorBase
	nodeName string
}

// NewFabricBindingCreatorReconciler returns a new BindingCreatorReconciler.
func NewFabricBindingCreatorReconciler(cl client.Client, s *runtime.Scheme, nodeName string) *BindingCreatorReconciler {
	return &BindingCreatorReconciler{
		BindingCreatorBase: firewallpkg.BindingCreatorBase{Client: cl, Scheme: s},
		nodeName:           nodeName,
	}
}

// Reconcile creates or updates the FirewallConfigurationBinding resource for the
// current fabric node referenced by the given FirewallConfiguration.
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

	klog.V(4).Infof("Reconciling fabric firewallconfiguration binding resources for %s", req.String())

	targets := []networkingv1beta1.TargetReference{{
		APIVersion: firewallpkg.TargetAPIVersionV1,
		Kind:       firewallpkg.TargetKindNode,
		Name:       r.nodeName,
	}}

	if err := r.ReconcileBindings(ctx, fwcfg, targets); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling fabric bindings for %s: %w", req.String(), err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers the FabricBindingCreatorReconciler with the manager.
// The labelsSets argument defines which FirewallConfiguration resources this fabric
// node is responsible for; only resources matching one of the sets are reconciled.
func (r *BindingCreatorReconciler) SetupWithManager(mgr ctrl.Manager, labelsSets []labels.Set) error {
	filterByLabelsPredicate := utilspredicates.NewAnyLabelsSetPredicate(labelsSets)

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlFabricFirewallConfigurationBindingCreator).
		For(&networkingv1beta1.FirewallConfiguration{}, builder.WithPredicates(filterByLabelsPredicate)).
		Owns(&networkingv1beta1.FirewallConfigurationBinding{}, builder.WithPredicates(
			firewallpkg.ForgeTargetRefPredicate(networkingv1beta1.TargetReference{
				APIVersion: firewallpkg.TargetAPIVersionV1,
				Kind:       firewallpkg.TargetKindNode,
				Name:       r.nodeName,
			}))).
		Complete(r)
}
