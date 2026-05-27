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

// Package firewall implements the controllers for the firewall resources.
package firewall

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/fabric"
	firewallpkg "github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/remapping"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// bindingTarget represents a single entity (node or gateway) that needs a FirewallConfigurationBinding.
type bindingTarget struct {
	// entityName is the node name (for fabric) or gateway name (for gateway).
	entityName string
	// bindingLabels are the labels to set on the FirewallConfigurationBinding resource.
	bindingLabels map[string]string
}

// BindingCreatorReconciler reconciles FirewallConfiguration resources and creates
// the corresponding FirewallConfigurationBinding resources for each target entity.
//
//nolint:revive // We usually use the name of the reconciled resource in the controller name.
type BindingCreatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewBindingCreatorReconciler returns a new BindingCreatorReconciler.
func NewBindingCreatorReconciler(cl client.Client, s *runtime.Scheme) *BindingCreatorReconciler {
	return &BindingCreatorReconciler{Client: cl, Scheme: s}
}

// Reconcile creates or deletes FirewallConfigurationBinding resources for each target entity
// referenced by the given FirewallConfiguration.
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

	klog.V(4).Infof("Reconciling firewallconfiguration binding resources for %s", req.String())

	targets, err := r.getTargets(ctx, fwcfg)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("computing binding targets for %s: %w", req.String(), err)
	}

	// Ensure a binding exists for each target.
	expectedNames := make(map[string]struct{}, len(targets))
	for i := range targets {
		t := &targets[i]
		bindingName := bindingResourceName(fwcfg.Name, t.entityName)
		expectedNames[bindingName] = struct{}{}
		if err := r.ensureBinding(ctx, fwcfg, bindingName, t.bindingLabels); err != nil {
			return ctrl.Result{}, fmt.Errorf("ensuring binding %s: %w", bindingName, err)
		}
	}

	// Delete bindings that no longer have a corresponding target.
	if err := r.deleteStaleBindings(ctx, fwcfg, expectedNames); err != nil {
		return ctrl.Result{}, fmt.Errorf("cleaning stale bindings for %s: %w", req.String(), err)
	}

	return ctrl.Result{}, nil
}

// getTargets derives the list of binding targets from the FirewallConfiguration labels.
func (r *BindingCreatorReconciler) getTargets(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration) ([]bindingTarget, error) {
	category := fwcfg.Labels[firewallpkg.FirewallCategoryTargetKey]
	subcategory := fwcfg.Labels[firewallpkg.FirewallSubCategoryTargetKey]
	unique := fwcfg.Labels[firewallpkg.FirewallUniqueTargetKey]

	switch category {
	case fabric.FirewallCategoryTargetValue:
		return r.getFabricTargets(ctx, subcategory, unique)
	case gateway.FirewallCategoryGwTargetValue:
		return r.getGatewayTargets(ctx, subcategory, unique)
	default:
		klog.V(4).Infof("FirewallConfiguration %s/%s has unknown category %q; skipping",
			fwcfg.Namespace, fwcfg.Name, category)
		return nil, nil
	}
}

// getFabricTargets enumerates targets for fabric-category FirewallConfigurations.
func (r *BindingCreatorReconciler) getFabricTargets(ctx context.Context, subcategory, unique string) ([]bindingTarget, error) {
	switch subcategory {
	case fabric.FirewallSubCategoryTargetAllNodesValue:
		return r.allInternalNodeTargets(ctx, fabric.ForgeFirewallBindingTargetLabels)
	case fabric.FirewallSubCategoryTargetSingleNodeValue:
		// Single-node: one binding for the specific node indicated by unique.
		return []bindingTarget{{
			entityName:    unique,
			bindingLabels: fabric.ForgeFirewallBindingTargetLabelsSingleNode(unique),
		}}, nil
	case remapping.FirewallSubCategoryTargetValueIPMapping:
		return r.allInternalNodeTargets(ctx, remapping.ForgeFirewallBindingTargetLabelsIPMappingFabric)
	default:
		klog.V(4).Infof("Unknown fabric subcategory %q; skipping", subcategory)
		return nil, nil
	}
}

// getGatewayTargets enumerates targets for gateway-category FirewallConfigurations.
func (r *BindingCreatorReconciler) getGatewayTargets(ctx context.Context,
	subcategory, unique string) ([]bindingTarget, error) {
	switch subcategory {
	case gateway.FirewallSubCategoryAllGatewaysTargetValue:
		return r.allGatewayTargets(ctx, gateway.ForgeFirewallBindingAllGatewaysTargetLabels)
	case gateway.FirewallSubCategoryFabricTargetValue:
		return r.allGatewayTargets(ctx, gateway.ForgeFirewallBindingInternalTargetLabels)
	case remapping.FirewallSubCategoryTargetValueIPMapping:
		return r.allGatewayTargets(ctx, remapping.ForgeFirewallBindingTargetLabelsIPMappingGw)
	case "":
		// No subcategory: remapping for a specific gateway identified by unique=remoteID.
		return r.singleGatewayTarget(ctx, unique)
	default:
		klog.V(4).Infof("Unknown gateway subcategory %q; skipping", subcategory)
		return nil, nil
	}
}

// allInternalNodeTargets lists all InternalNodes and builds one target per node.
func (r *BindingCreatorReconciler) allInternalNodeTargets(ctx context.Context,
	forgeLabels func(nodeName string) map[string]string) ([]bindingTarget, error) {
	nodeList := &networkingv1beta1.InternalNodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return nil, fmt.Errorf("listing internalnodes: %w", err)
	}
	targets := make([]bindingTarget, 0, len(nodeList.Items))
	for i := range nodeList.Items {
		name := nodeList.Items[i].Name
		targets = append(targets, bindingTarget{
			entityName:    name,
			bindingLabels: forgeLabels(name),
		})
	}
	return targets, nil
}

// allGatewayTargets lists all GatewayServer and GatewayClient across all namespaces
// and builds one target per gateway. Gateways live in tenant namespaces (liqo-tenant-*),
// not necessarily in the same namespace as the FirewallConfiguration.
func (r *BindingCreatorReconciler) allGatewayTargets(ctx context.Context,
	forgeLabels func(gwName string) map[string]string) ([]bindingTarget, error) {
	var targets []bindingTarget

	serverList := &networkingv1beta1.GatewayServerList{}
	if err := r.List(ctx, serverList); err != nil {
		return nil, fmt.Errorf("listing gatewayservers: %w", err)
	}
	for i := range serverList.Items {
		name := serverList.Items[i].Name
		targets = append(targets, bindingTarget{
			entityName:    name,
			bindingLabels: forgeLabels(name),
		})
	}

	clientList := &networkingv1beta1.GatewayClientList{}
	if err := r.List(ctx, clientList); err != nil {
		return nil, fmt.Errorf("listing gatewayclients: %w", err)
	}
	for i := range clientList.Items {
		name := clientList.Items[i].Name
		targets = append(targets, bindingTarget{
			entityName:    name,
			bindingLabels: forgeLabels(name),
		})
	}

	return targets, nil
}

// singleGatewayTarget finds the specific GatewayServer or GatewayClient for the given remoteID.
func (r *BindingCreatorReconciler) singleGatewayTarget(ctx context.Context, remoteID string) ([]bindingTarget, error) {
	gwServer, gwClient, err := getters.GetGatewaysByClusterID(ctx, r.Client, liqov1beta1.ClusterID(remoteID))
	if err != nil {
		return nil, fmt.Errorf("getting gateways for remoteID %q: %w", remoteID, err)
	}

	var gwName string
	switch {
	case gwServer != nil:
		gwName = gwServer.Name
	case gwClient != nil:
		gwName = gwClient.Name
	default:
		klog.V(4).Infof("No gateway found for remoteID %q; skipping binding creation", remoteID)
		return nil, nil
	}

	return []bindingTarget{{
		entityName:    gwName,
		bindingLabels: remapping.ForgeFirewallBindingTargetLabels(remoteID, gwName),
	}}, nil
}

// ensureBinding creates or updates a FirewallConfigurationBinding for the given entity.
func (r *BindingCreatorReconciler) ensureBinding(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration, bindingName string, labels map[string]string) error {
	binding := &networkingv1beta1.FirewallConfigurationBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: fwcfg.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, binding, func() error {
		binding.Labels = labels
		binding.Spec.FirewallConfigurationRef = corev1.LocalObjectReference{Name: fwcfg.Name}
		return controllerutil.SetControllerReference(fwcfg, binding, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("creating/updating binding %s/%s: %w", fwcfg.Namespace, bindingName, err)
	}
	if op != controllerutil.OperationResultNone {
		klog.V(4).Infof("FirewallConfigurationBinding %s/%s %s", fwcfg.Namespace, bindingName, op)
	}
	return nil
}

// deleteStaleBindings removes FirewallConfigurationBinding resources owned by fwcfg
// whose names are not in expectedNames.
func (r *BindingCreatorReconciler) deleteStaleBindings(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration, expectedNames map[string]struct{}) error {
	bindingList := &networkingv1beta1.FirewallConfigurationBindingList{}
	if err := r.List(ctx, bindingList, client.InNamespace(fwcfg.Namespace)); err != nil {
		return fmt.Errorf("listing firewallconfigurationbindings: %w", err)
	}

	for i := range bindingList.Items {
		binding := &bindingList.Items[i]
		if !isOwnedBy(binding, fwcfg.UID) {
			continue
		}
		if _, ok := expectedNames[binding.Name]; ok {
			continue
		}
		klog.V(4).Infof("Deleting stale FirewallConfigurationBinding %s/%s", binding.Namespace, binding.Name)
		// Remove the gateway-pod finalizer before deleting so the resource is not stuck if the
		// gateway pod is already gone (the pod cannot remove its own finalizer when it is dead).
		if controllerutil.ContainsFinalizer(binding, firewallpkg.FirewallConfigurationBindingControllerFinalizer) {
			original := binding.DeepCopy()
			controllerutil.RemoveFinalizer(binding, firewallpkg.FirewallConfigurationBindingControllerFinalizer)
			if err := r.Patch(ctx, binding, client.MergeFrom(original)); err != nil {
				if !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
					return fmt.Errorf("removing finalizer from stale binding %s/%s: %w", binding.Namespace, binding.Name, err)
				}
			}
		}
		if err := r.Delete(ctx, binding); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting binding %s/%s: %w", binding.Namespace, binding.Name, err)
		}
	}
	return nil
}

// isOwnedBy reports whether obj has an ownerReference with the given UID.
func isOwnedBy(obj metav1.Object, ownerUID types.UID) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == ownerUID {
			return true
		}
	}
	return false
}

// bindingResourceName returns the deterministic name for a FirewallConfigurationBinding.
func bindingResourceName(fwcfgName, entityName string) string {
	name := fmt.Sprintf("%s-%s", fwcfgName, entityName)
	if len(name) > 253 {
		// Preserve the entity suffix; truncate the fwcfg prefix.
		prefixLen := 253 - 1 - len(entityName)
		name = fmt.Sprintf("%s-%s", fwcfgName[:prefixLen], entityName)
	}
	return name
}

// SetupWithManager registers the BindingCreatorReconciler with the manager.
func (r *BindingCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fabricFWCfgMapper := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return r.enqueueFirewallConfigurationsByCategory(ctx, fabric.FirewallCategoryTargetValue)
	}
	gatewayFWCfgMapper := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return r.enqueueFirewallConfigurationsByCategory(ctx, gateway.FirewallCategoryGwTargetValue)
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlFirewallConfigurationBindingCreator).
		For(&networkingv1beta1.FirewallConfiguration{}).
		Owns(&networkingv1beta1.FirewallConfigurationBinding{}).
		Watches(&networkingv1beta1.InternalNode{},
			handler.EnqueueRequestsFromMapFunc(fabricFWCfgMapper)).
		Watches(&networkingv1beta1.GatewayServer{},
			handler.EnqueueRequestsFromMapFunc(gatewayFWCfgMapper)).
		Watches(&networkingv1beta1.GatewayClient{},
			handler.EnqueueRequestsFromMapFunc(gatewayFWCfgMapper)).
		Complete(r)
}

// enqueueFirewallConfigurationsByCategory lists all FirewallConfiguration resources with the
// given category label and returns reconcile requests for each.
func (r *BindingCreatorReconciler) enqueueFirewallConfigurationsByCategory(
	ctx context.Context, category string) []reconcile.Request {
	fwcfgList := &networkingv1beta1.FirewallConfigurationList{}
	if err := r.List(ctx, fwcfgList, client.MatchingLabels{
		firewallpkg.FirewallCategoryTargetKey: category,
	}); err != nil {
		klog.Errorf("Unable to list FirewallConfigurations with category %q: %v", category, err)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(fwcfgList.Items))
	for i := range fwcfgList.Items {
		item := &fwcfgList.Items[i]
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: item.Name, Namespace: item.Namespace},
		})
	}
	return requests
}
