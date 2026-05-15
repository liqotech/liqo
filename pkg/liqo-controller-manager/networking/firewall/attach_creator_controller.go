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

// attachTarget represents a single entity (node or gateway) that needs a FirewallConfigurationAttach.
type attachTarget struct {
	// entityName is the node name (for fabric) or gateway name (for gateway).
	entityName string
	// attachLabels are the labels to set on the FirewallConfigurationAttach resource.
	attachLabels map[string]string
}

// AttachCreatorReconciler reconciles FirewallConfiguration resources and creates
// the corresponding FirewallConfigurationAttach resources for each target entity.
//
//nolint:revive // We usually use the name of the reconciled resource in the controller name.
type AttachCreatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewAttachCreatorReconciler returns a new AttachCreatorReconciler.
func NewAttachCreatorReconciler(cl client.Client, s *runtime.Scheme) *AttachCreatorReconciler {
	return &AttachCreatorReconciler{Client: cl, Scheme: s}
}

// Reconcile creates or deletes FirewallConfigurationAttach resources for each target entity
// referenced by the given FirewallConfiguration.
func (r *AttachCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	fwcfg := &networkingv1beta1.FirewallConfiguration{}
	if err := r.Get(ctx, req.NamespacedName, fwcfg); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting firewallconfiguration: %w", err)
	}

	if !fwcfg.DeletionTimestamp.IsZero() {
		// FWCfg is being deleted; GC will handle the owned attaches via ownerRef.
		return ctrl.Result{}, nil
	}

	klog.V(4).Infof("Reconciling firewallconfiguration attach resources for %s", req.String())

	targets, err := r.getTargets(ctx, fwcfg)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("computing attach targets for %s: %w", req.String(), err)
	}

	// Ensure an attach exists for each target.
	expectedNames := make(map[string]struct{}, len(targets))
	for i := range targets {
		t := &targets[i]
		attachName := attachResourceName(fwcfg.Name, t.entityName)
		expectedNames[attachName] = struct{}{}
		if err := r.ensureAttach(ctx, fwcfg, attachName, t.attachLabels); err != nil {
			return ctrl.Result{}, fmt.Errorf("ensuring attach %s: %w", attachName, err)
		}
	}

	// Delete attaches that no longer have a corresponding target.
	if err := r.deleteStaleAttaches(ctx, fwcfg, expectedNames); err != nil {
		return ctrl.Result{}, fmt.Errorf("cleaning stale attaches for %s: %w", req.String(), err)
	}

	return ctrl.Result{}, nil
}

// getTargets derives the list of attach targets from the FirewallConfiguration labels.
func (r *AttachCreatorReconciler) getTargets(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration) ([]attachTarget, error) {
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
func (r *AttachCreatorReconciler) getFabricTargets(ctx context.Context, subcategory, unique string) ([]attachTarget, error) {
	switch subcategory {
	case fabric.FirewallSubCategoryTargetAllNodesValue:
		return r.allInternalNodeTargets(ctx, func(nodeName string) map[string]string {
			return fabric.ForgeFirewallAttachTargetLabels(nodeName)
		})
	case fabric.FirewallSubCategoryTargetSingleNodeValue:
		// Single-node: one attach for the specific node indicated by unique.
		return []attachTarget{{
			entityName:   unique,
			attachLabels: fabric.ForgeFirewallAttachTargetLabelsSingleNode(unique),
		}}, nil
	case remapping.FirewallSubCategoryTargetValueIPMapping:
		return r.allInternalNodeTargets(ctx, func(nodeName string) map[string]string {
			return remapping.ForgeFirewallAttachTargetLabelsIPMappingFabric(nodeName)
		})
	default:
		klog.V(4).Infof("Unknown fabric subcategory %q; skipping", subcategory)
		return nil, nil
	}
}

// getGatewayTargets enumerates targets for gateway-category FirewallConfigurations.
func (r *AttachCreatorReconciler) getGatewayTargets(ctx context.Context,
	subcategory, unique string) ([]attachTarget, error) {
	switch subcategory {
	case gateway.FirewallSubCategoryAllGatewaysTargetValue:
		return r.allGatewayTargets(ctx, func(gwName string) map[string]string {
			return gateway.ForgeFirewallAttachAllGatewaysTargetLabels(gwName)
		})
	case gateway.FirewallSubCategoryFabricTargetValue:
		return r.allGatewayTargets(ctx, func(gwName string) map[string]string {
			return gateway.ForgeFirewallAttachInternalTargetLabels(gwName)
		})
	case remapping.FirewallSubCategoryTargetValueIPMapping:
		return r.allGatewayTargets(ctx, func(gwName string) map[string]string {
			return remapping.ForgeFirewallAttachTargetLabelsIPMappingGw(gwName)
		})
	case "":
		// No subcategory: remapping for a specific gateway identified by unique=remoteID.
		return r.singleGatewayTarget(ctx, unique)
	default:
		klog.V(4).Infof("Unknown gateway subcategory %q; skipping", subcategory)
		return nil, nil
	}
}

// allInternalNodeTargets lists all InternalNodes and builds one target per node.
func (r *AttachCreatorReconciler) allInternalNodeTargets(ctx context.Context,
	forgeLabels func(nodeName string) map[string]string) ([]attachTarget, error) {
	nodeList := &networkingv1beta1.InternalNodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return nil, fmt.Errorf("listing internalnodes: %w", err)
	}
	targets := make([]attachTarget, 0, len(nodeList.Items))
	for i := range nodeList.Items {
		name := nodeList.Items[i].Name
		targets = append(targets, attachTarget{
			entityName:   name,
			attachLabels: forgeLabels(name),
		})
	}
	return targets, nil
}

// allGatewayTargets lists all GatewayServer and GatewayClient across all namespaces
// and builds one target per gateway. Gateways live in tenant namespaces (liqo-tenant-*),
// not necessarily in the same namespace as the FirewallConfiguration.
func (r *AttachCreatorReconciler) allGatewayTargets(ctx context.Context,
	forgeLabels func(gwName string) map[string]string) ([]attachTarget, error) {
	var targets []attachTarget

	serverList := &networkingv1beta1.GatewayServerList{}
	if err := r.List(ctx, serverList); err != nil {
		return nil, fmt.Errorf("listing gatewayservers: %w", err)
	}
	for i := range serverList.Items {
		name := serverList.Items[i].Name
		targets = append(targets, attachTarget{
			entityName:   name,
			attachLabels: forgeLabels(name),
		})
	}

	clientList := &networkingv1beta1.GatewayClientList{}
	if err := r.List(ctx, clientList); err != nil {
		return nil, fmt.Errorf("listing gatewayclients: %w", err)
	}
	for i := range clientList.Items {
		name := clientList.Items[i].Name
		targets = append(targets, attachTarget{
			entityName:   name,
			attachLabels: forgeLabels(name),
		})
	}

	return targets, nil
}

// singleGatewayTarget finds the specific GatewayServer or GatewayClient for the given remoteID.
func (r *AttachCreatorReconciler) singleGatewayTarget(ctx context.Context, remoteID string) ([]attachTarget, error) {
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
		klog.V(4).Infof("No gateway found for remoteID %q; skipping attach creation", remoteID)
		return nil, nil
	}

	return []attachTarget{{
		entityName:   gwName,
		attachLabels: remapping.ForgeFirewallAttachTargetLabels(remoteID, gwName),
	}}, nil
}

// ensureAttach creates or updates a FirewallConfigurationAttach for the given entity.
func (r *AttachCreatorReconciler) ensureAttach(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration, attachName string, labels map[string]string) error {
	attach := &networkingv1beta1.FirewallConfigurationAttach{
		ObjectMeta: metav1.ObjectMeta{
			Name:      attachName,
			Namespace: fwcfg.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, attach, func() error {
		attach.Labels = labels
		attach.Spec.FirewallConfigurationRef = corev1.LocalObjectReference{Name: fwcfg.Name}
		return controllerutil.SetControllerReference(fwcfg, attach, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("creating/updating attach %s/%s: %w", fwcfg.Namespace, attachName, err)
	}
	if op != controllerutil.OperationResultNone {
		klog.V(4).Infof("FirewallConfigurationAttach %s/%s %s", fwcfg.Namespace, attachName, op)
	}
	return nil
}

// deleteStaleAttaches removes FirewallConfigurationAttach resources owned by fwcfg
// whose names are not in expectedNames.
func (r *AttachCreatorReconciler) deleteStaleAttaches(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration, expectedNames map[string]struct{}) error {
	attachList := &networkingv1beta1.FirewallConfigurationAttachList{}
	if err := r.List(ctx, attachList, client.InNamespace(fwcfg.Namespace)); err != nil {
		return fmt.Errorf("listing firewallconfigurationattaches: %w", err)
	}

	for i := range attachList.Items {
		attach := &attachList.Items[i]
		if !isOwnedBy(attach, fwcfg.UID) {
			continue
		}
		if _, ok := expectedNames[attach.Name]; ok {
			continue
		}
		klog.V(4).Infof("Deleting stale FirewallConfigurationAttach %s/%s", attach.Namespace, attach.Name)
		// Remove the gateway-pod finalizer before deleting so the resource is not stuck if the
		// gateway pod is already gone (the pod cannot remove its own finalizer when it is dead).
		if controllerutil.ContainsFinalizer(attach, firewallpkg.FirewallConfigurationAttachControllerFinalizer) {
			original := attach.DeepCopy()
			controllerutil.RemoveFinalizer(attach, firewallpkg.FirewallConfigurationAttachControllerFinalizer)
			if err := r.Patch(ctx, attach, client.MergeFrom(original)); err != nil {
				if !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
					return fmt.Errorf("removing finalizer from stale attach %s/%s: %w", attach.Namespace, attach.Name, err)
				}
			}
		}
		if err := r.Delete(ctx, attach); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting attach %s/%s: %w", attach.Namespace, attach.Name, err)
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

// attachResourceName returns the deterministic name for a FirewallConfigurationAttach.
func attachResourceName(fwcfgName, entityName string) string {
	name := fmt.Sprintf("%s-%s", fwcfgName, entityName)
	if len(name) > 253 {
		// Preserve the entity suffix; truncate the fwcfg prefix.
		prefixLen := 253 - 1 - len(entityName)
		name = fmt.Sprintf("%s-%s", fwcfgName[:prefixLen], entityName)
	}
	return name
}

// SetupWithManager registers the AttachCreatorReconciler with the manager.
func (r *AttachCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fabricFWCfgMapper := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return r.enqueueFirewallConfigurationsByCategory(ctx, fabric.FirewallCategoryTargetValue)
	}
	gatewayFWCfgMapper := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return r.enqueueFirewallConfigurationsByCategory(ctx, gateway.FirewallCategoryGwTargetValue)
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlFirewallConfigurationAttachCreator).
		For(&networkingv1beta1.FirewallConfiguration{}).
		Owns(&networkingv1beta1.FirewallConfigurationAttach{}).
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
func (r *AttachCreatorReconciler) enqueueFirewallConfigurationsByCategory(
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
