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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	firewallpkg "github.com/liqotech/liqo/pkg/firewall"
)

// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch

// FabricBindingCreatorReconciler reconciles FirewallConfiguration resources with the
// fabric category and creates the corresponding FirewallConfigurationBinding resources
// for each InternalNode.
type FabricBindingCreatorReconciler struct {
	firewallpkg.BindingCreatorBase
}

// NewFabricBindingCreatorReconciler returns a new FabricBindingCreatorReconciler.
func NewFabricBindingCreatorReconciler(cl client.Client, s *runtime.Scheme) *FabricBindingCreatorReconciler {
	return &FabricBindingCreatorReconciler{
		BindingCreatorBase: firewallpkg.BindingCreatorBase{Client: cl, Scheme: s},
	}
}

// Reconcile creates or deletes FirewallConfigurationBinding resources for each InternalNode
// referenced by the given fabric-category FirewallConfiguration.
func (r *FabricBindingCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	// Only handle fabric-category FirewallConfigurations.
	category := fwcfg.Labels[firewallpkg.FirewallCategoryTargetKey]
	if category != FirewallCategoryTargetValue {
		return ctrl.Result{}, nil
	}

	klog.V(4).Infof("Reconciling fabric firewallconfiguration binding resources for %s", req.String())

	targets, err := r.getFabricTargets(ctx, fwcfg)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("computing fabric binding targets for %s: %w", req.String(), err)
	}

	if err := r.ReconcileBindings(ctx, fwcfg, targets); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling fabric bindings for %s: %w", req.String(), err)
	}

	return ctrl.Result{}, nil
}

// getFabricTargets enumerates targets for fabric-category FirewallConfigurations.
func (r *FabricBindingCreatorReconciler) getFabricTargets(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration) ([]firewallpkg.BindingTarget, error) {
	subcategory := fwcfg.Labels[firewallpkg.FirewallSubCategoryTargetKey]
	unique := fwcfg.Labels[firewallpkg.FirewallUniqueTargetKey]

	switch subcategory {
	case FirewallSubCategoryTargetAllNodesValue:
		return r.allInternalNodeTargets(ctx, ForgeFirewallBindingTargetLabels)
	case FirewallSubCategoryTargetSingleNodeValue:
		// Single-node: one binding for the specific node indicated by unique.
		return []firewallpkg.BindingTarget{{
			EntityName:    unique,
			BindingLabels: ForgeFirewallBindingTargetLabelsSingleNode(unique),
		}}, nil
	case firewallpkg.FirewallSubCategoryTargetValueIPMapping:
		return r.allInternalNodeTargets(ctx, firewallpkg.ForgeFirewallBindingTargetLabelsIPMappingFabric)
	default:
		klog.V(4).Infof("Unknown fabric subcategory %q; skipping", subcategory)
		return nil, nil
	}
}

// allInternalNodeTargets lists all InternalNodes and builds one target per node.
func (r *FabricBindingCreatorReconciler) allInternalNodeTargets(ctx context.Context,
	forgeLabels func(nodeName string) map[string]string) ([]firewallpkg.BindingTarget, error) {
	nodeList := &networkingv1beta1.InternalNodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return nil, fmt.Errorf("listing internalnodes: %w", err)
	}
	targets := make([]firewallpkg.BindingTarget, 0, len(nodeList.Items))
	for i := range nodeList.Items {
		name := nodeList.Items[i].Name
		targets = append(targets, firewallpkg.BindingTarget{
			EntityName:    name,
			BindingLabels: forgeLabels(name),
		})
	}
	return targets, nil
}

// SetupWithManager registers the FabricBindingCreatorReconciler with the manager.
func (r *FabricBindingCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fabricFWCfgMapper := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return r.EnqueueFirewallConfigurationsByCategory(ctx, FirewallCategoryTargetValue)
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlFabricFirewallConfigurationBindingCreator).
		For(&networkingv1beta1.FirewallConfiguration{}).
		Owns(&networkingv1beta1.FirewallConfigurationBinding{}).
		Watches(&networkingv1beta1.InternalNode{},
			handler.EnqueueRequestsFromMapFunc(fabricFWCfgMapper)).
		Complete(r)
}
