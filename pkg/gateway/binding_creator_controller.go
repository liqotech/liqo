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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	firewallpkg "github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurationbindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers;gatewayclients,verbs=get;list;watch

// GatewayBindingCreatorReconciler reconciles FirewallConfiguration resources with the
// gateway category and creates the corresponding FirewallConfigurationBinding resources
// for each GatewayServer and GatewayClient.
type GatewayBindingCreatorReconciler struct {
	firewallpkg.BindingCreatorBase
}

// NewGatewayBindingCreatorReconciler returns a new GatewayBindingCreatorReconciler.
func NewGatewayBindingCreatorReconciler(cl client.Client, s *runtime.Scheme) *GatewayBindingCreatorReconciler {
	return &GatewayBindingCreatorReconciler{
		BindingCreatorBase: firewallpkg.BindingCreatorBase{Client: cl, Scheme: s},
	}
}

// Reconcile creates or deletes FirewallConfigurationBinding resources for each gateway
// referenced by the given gateway-category FirewallConfiguration.
func (r *GatewayBindingCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

	// Only handle gateway-category FirewallConfigurations.
	category := fwcfg.Labels[firewallpkg.FirewallCategoryTargetKey]
	if category != FirewallCategoryGwTargetValue {
		return ctrl.Result{}, nil
	}

	klog.V(4).Infof("Reconciling gateway firewallconfiguration binding resources for %s", req.String())

	targets, err := r.getGatewayTargets(ctx, fwcfg)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("computing gateway binding targets for %s: %w", req.String(), err)
	}

	if err := r.ReconcileBindings(ctx, fwcfg, targets); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling gateway bindings for %s: %w", req.String(), err)
	}

	return ctrl.Result{}, nil
}

// getGatewayTargets enumerates targets for gateway-category FirewallConfigurations.
func (r *GatewayBindingCreatorReconciler) getGatewayTargets(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration) ([]firewallpkg.BindingTarget, error) {
	subcategory := fwcfg.Labels[firewallpkg.FirewallSubCategoryTargetKey]
	unique := fwcfg.Labels[firewallpkg.FirewallUniqueTargetKey]

	switch subcategory {
	case FirewallSubCategoryAllGatewaysTargetValue:
		return r.allGatewayTargets(ctx, ForgeFirewallBindingAllGatewaysTargetLabels)
	case FirewallSubCategoryFabricTargetValue:
		return r.allGatewayTargets(ctx, ForgeFirewallBindingInternalTargetLabels)
	case firewallpkg.FirewallSubCategoryTargetValueIPMapping:
		return r.allGatewayTargets(ctx, firewallpkg.ForgeFirewallBindingTargetLabelsIPMappingGw)
	case "":
		// No subcategory: remapping for a specific gateway identified by unique=remoteID.
		return r.singleGatewayTarget(ctx, unique)
	default:
		klog.V(4).Infof("Unknown gateway subcategory %q; skipping", subcategory)
		return nil, nil
	}
}

// allGatewayTargets lists all GatewayServer and GatewayClient across all namespaces
// and builds one target per gateway. Gateways live in tenant namespaces (liqo-tenant-*),
// not necessarily in the same namespace as the FirewallConfiguration.
func (r *GatewayBindingCreatorReconciler) allGatewayTargets(ctx context.Context,
	forgeLabels func(gwName string) map[string]string) ([]firewallpkg.BindingTarget, error) {
	var targets []firewallpkg.BindingTarget

	serverList := &networkingv1beta1.GatewayServerList{}
	if err := r.List(ctx, serverList); err != nil {
		return nil, fmt.Errorf("listing gatewayservers: %w", err)
	}
	for i := range serverList.Items {
		name := serverList.Items[i].Name
		targets = append(targets, firewallpkg.BindingTarget{
			EntityName:    name,
			BindingLabels: forgeLabels(name),
		})
	}

	clientList := &networkingv1beta1.GatewayClientList{}
	if err := r.List(ctx, clientList); err != nil {
		return nil, fmt.Errorf("listing gatewayclients: %w", err)
	}
	for i := range clientList.Items {
		name := clientList.Items[i].Name
		targets = append(targets, firewallpkg.BindingTarget{
			EntityName:    name,
			BindingLabels: forgeLabels(name),
		})
	}

	return targets, nil
}

// singleGatewayTarget finds the specific GatewayServer or GatewayClient for the given remoteID.
func (r *GatewayBindingCreatorReconciler) singleGatewayTarget(ctx context.Context, remoteID string) ([]firewallpkg.BindingTarget, error) {
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

	return []firewallpkg.BindingTarget{{
		EntityName:    gwName,
		BindingLabels: firewallpkg.ForgeFirewallBindingTargetLabelsRemapping(remoteID, gwName),
	}}, nil
}

// SetupWithManager registers the GatewayBindingCreatorReconciler with the manager.
func (r *GatewayBindingCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gatewayFWCfgMapper := func(ctx context.Context, _ client.Object) []reconcile.Request {
		return r.EnqueueFirewallConfigurationsByCategory(ctx, FirewallCategoryGwTargetValue)
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlGatewayFirewallConfigurationBindingCreator).
		For(&networkingv1beta1.FirewallConfiguration{}).
		Owns(&networkingv1beta1.FirewallConfigurationBinding{}).
		Watches(&networkingv1beta1.GatewayServer{},
			handler.EnqueueRequestsFromMapFunc(gatewayFWCfgMapper)).
		Watches(&networkingv1beta1.GatewayClient{},
			handler.EnqueueRequestsFromMapFunc(gatewayFWCfgMapper)).
		Complete(r)
}
