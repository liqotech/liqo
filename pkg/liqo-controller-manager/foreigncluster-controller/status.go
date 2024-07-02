// Copyright 2019-2024 The Liqo Authors
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

package foreignclustercontroller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1alpha1 "github.com/liqotech/liqo/apis/core/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/gateway/forge"
	"github.com/liqotech/liqo/pkg/utils"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/pod"
)

func (r *ForeignClusterReconciler) clearStatusExceptConditions(foreignCluster *liqov1alpha1.ForeignCluster) {
	foreignCluster.Status = liqov1alpha1.ForeignClusterStatus{
		Role: liqov1alpha1.UnknownRole,
		Modules: liqov1alpha1.Modules{
			Networking: liqov1alpha1.Module{
				Enabled:    false,
				Conditions: foreignCluster.Status.Modules.Networking.Conditions,
			},
			Authentication: liqov1alpha1.Module{
				Enabled:    false,
				Conditions: foreignCluster.Status.Modules.Authentication.Conditions,
			},
			Offloading: liqov1alpha1.Module{
				Enabled:    false,
				Conditions: foreignCluster.Status.Modules.Offloading.Conditions,
			},
		},
		Conditions: foreignCluster.Status.Conditions,
	}
}

func (r *ForeignClusterReconciler) handleNetworkingModuleStatus(ctx context.Context,
	fc *liqov1alpha1.ForeignCluster, moduleEnabled bool) error {
	if !moduleEnabled {
		clearModule(&fc.Status.Modules.Networking)
		return nil
	}

	clusterID := fc.Spec.ClusterID

	connection, err := getters.GetConnectionByClusterID(ctx, r.Client, string(clusterID))
	switch {
	case errors.IsNotFound(err):
		klog.V(6).Infof("Connection resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Networking, liqov1alpha1.NetworkConnectionStatusCondition)
	case err != nil:
		klog.Errorf("an error occurred while getting the Connection resource for the ForeignCluster %q: %s", clusterID, err)
		return err
	default:
		fcutils.EnableModuleNetworking(fc)
		switch connection.Status.Value {
		case networkingv1alpha1.Connected:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1alpha1.NetworkConnectionStatusCondition, liqov1alpha1.ConditionStatusEstablished,
				connectionEstablishedReason, connectionEstablishedMessage)
		case networkingv1alpha1.Connecting:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1alpha1.NetworkConnectionStatusCondition, liqov1alpha1.ConditionStatusPending,
				connectionPendingReason, connectionPendingMessage)
		default:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1alpha1.NetworkConnectionStatusCondition, liqov1alpha1.ConditionStatusError,
				connectionErrorReason, connectionErrorMessage)
		}
	}

	gwServer, err := getters.GetGatewayServerByClusterID(ctx, r.Client, clusterID)
	switch {
	case errors.IsNotFound(err):
		klog.V(6).Infof("GatewayServer resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Networking, liqov1alpha1.NetworkGatewayServerStatusCondition)
	case err != nil:
		klog.Errorf("an error occurred while getting the GatewayServer resource for the ForeignCluster %q: %s", clusterID, err)
		return err
	default:
		fcutils.EnableModuleNetworking(fc)
		gwDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      forge.GatewayResourceName(gwServer.GetName()),
				Namespace: gwServer.GetNamespace(),
			},
		}
		if err := r.Get(ctx, client.ObjectKeyFromObject(gwDeployment), gwDeployment); err != nil {
			klog.Errorf("an error occurred while getting the GatewayServer deployment for the ForeignCluster %q: %s", clusterID, err)
			return err
		}
		switch {
		case gwDeployment.Status.ReadyReplicas == 0:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1alpha1.NetworkGatewayServerStatusCondition, liqov1alpha1.ConditionStatusNotReady,
				gatewaysNotReadyReason, gatewaysNotReadyMessage)
		case gwDeployment.Status.UnavailableReplicas > 0:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1alpha1.NetworkGatewayServerStatusCondition, liqov1alpha1.ConditionStatusSomeNotReady,
				gatewaySomeNotReadyReason, gatewaySomeNotReadyMessage)
		default:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1alpha1.NetworkGatewayServerStatusCondition, liqov1alpha1.ConditionStatusReady,
				gatewaysReadyReason, gatewaysReadyMessage)
		}
	}

	gwClient, err := getters.GetGatewayClientByClusterID(ctx, r.Client, clusterID)
	switch {
	case errors.IsNotFound(err):
		klog.V(6).Infof("GatewayClient resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Networking, liqov1alpha1.NetworkGatewayClientStatusCondition)
	case err != nil:
		klog.Errorf("an error occurred while getting the GatewayClient resource for the ForeignCluster %q: %s", clusterID, err)
		return err
	default:
		fcutils.EnableModuleNetworking(fc)
		gwDeployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      forge.GatewayResourceName(gwClient.GetName()),
				Namespace: gwClient.GetNamespace(),
			},
		}
		if err := r.Get(ctx, client.ObjectKeyFromObject(gwDeployment), gwDeployment); err != nil {
			klog.Errorf("an error occurred while getting the GatewayClient deployment for the ForeignCluster %q: %s", clusterID, err)
			return err
		}
		switch {
		case gwDeployment.Status.ReadyReplicas == 0:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1alpha1.NetworkGatewayClientStatusCondition, liqov1alpha1.ConditionStatusNotReady,
				gatewaysNotReadyReason, gatewaysNotReadyMessage)
		case gwDeployment.Status.UnavailableReplicas > 0:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1alpha1.NetworkGatewayClientStatusCondition, liqov1alpha1.ConditionStatusSomeNotReady,
				gatewaySomeNotReadyReason, gatewaySomeNotReadyMessage)
		default:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1alpha1.NetworkGatewayClientStatusCondition, liqov1alpha1.ConditionStatusReady,
				gatewaysReadyReason, gatewaysReadyMessage)
		}
	}

	return nil
}

func (r *ForeignClusterReconciler) handleAuthenticationModuleStatus(ctx context.Context,
	fc *liqov1alpha1.ForeignCluster, moduleEnabled bool, consumer, provider *bool) error {
	if !moduleEnabled {
		clearModule(&fc.Status.Modules.Authentication)
		return nil
	}

	clusterID := fc.Spec.ClusterID

	// Check if a Tenant resource for this cluser exists.
	tenant, err := getters.GetTenantByClusterID(ctx, r.Client, clusterID)
	switch {
	case errors.IsNotFound(err):
		klog.V(6).Infof("Tenant resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Authentication, liqov1alpha1.AuthTenantStatusCondition)
	case err != nil:
		klog.Errorf("an error occurred while getting the Tenant resource for the ForeignCluster %q: %s", clusterID, err)
		return err
	default:
		*consumer = true
		fcutils.EnableModuleAuthentication(fc)
		if tenant.Status.TenantNamespace != "" {
			fc.Status.TenantNamespace.Local = tenant.Status.TenantNamespace
		}

		if tenant.Status.AuthParams == nil || tenant.Status.TenantNamespace == "" {
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Authentication,
				liqov1alpha1.AuthTenantStatusCondition, liqov1alpha1.ConditionStatusNotReady,
				tenantNotReadyReason, tenantNotReadyMessage)
		} else {
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Authentication,
				liqov1alpha1.AuthTenantStatusCondition, liqov1alpha1.ConditionStatusReady,
				tenantReadyReason, tenantReadyMessage)
		}
	}

	// Check if an Identity resource of type ControlPlane for this cluster exists.
	identity, err := getters.GetControlPlaneIdentityByClusterID(ctx, r.Client, clusterID)
	switch {
	case errors.IsNotFound(err):
		klog.V(6).Infof("Identity resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Authentication, liqov1alpha1.AuthIdentityControlPlaneStatusCondition)
	case err != nil:
		klog.Errorf("an error occurred while getting the Identity resource for the ForeignCluster %q: %s", clusterID, err)
		return err
	default:
		*provider = true
		fcutils.EnableModuleAuthentication(fc)
		if identity.Spec.AuthParams.APIServer != "" {
			fc.Status.APIServerURL = identity.Spec.AuthParams.APIServer
		}
		if identity.Spec.AuthParams.ProxyURL != nil && *identity.Spec.AuthParams.ProxyURL != "" {
			fc.Status.ForeignProxyURL = *identity.Spec.AuthParams.ProxyURL
		}
		fc.Status.TenantNamespace.Local = identity.GetNamespace()
		if identity.Spec.Namespace != nil && *identity.Spec.Namespace != "" {
			fc.Status.TenantNamespace.Remote = *identity.Spec.Namespace
		}

		if identity.Status.KubeconfigSecretRef == nil || identity.Status.KubeconfigSecretRef.Name == "" {
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Authentication,
				liqov1alpha1.AuthIdentityControlPlaneStatusCondition, liqov1alpha1.ConditionStatusNotReady,
				identityNotReadyReason, identityNotReadyMessage)
		} else {
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Authentication,
				liqov1alpha1.AuthIdentityControlPlaneStatusCondition, liqov1alpha1.ConditionStatusReady,
				identityReadyReason, identityReadyMessage)
		}
	}

	return nil
}

func (r *ForeignClusterReconciler) handleOffloadingModuleStatus(ctx context.Context,
	fc *liqov1alpha1.ForeignCluster, moduleEnabled bool, provider *bool) error {
	if !moduleEnabled {
		clearModule(&fc.Status.Modules.Offloading)
		return nil
	}

	clusterID := fc.Spec.ClusterID

	// Get VirtualNodes for this cluster
	virtualNodes, err := getters.ListVirtualNodesByClusterID(ctx, r.Client, clusterID)
	if err != nil {
		klog.Errorf("an error occurred while listing VirtualNodes for the ForeignCluster %q: %s", clusterID, err)
		return err
	}

	if len(virtualNodes) == 0 {
		klog.V(6).Infof("No VirtualNodes found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Offloading, liqov1alpha1.OffloadingVirtualNodeStatusCondition)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Offloading, liqov1alpha1.OffloadingNodeStatusCondition)
		return nil
	}

	*provider = true
	fcutils.EnableModuleOffloading(fc)

	// Calculate the number of ready VirtualNodes and associated Nodes:
	// - a VirtualNode is ready if all its VirtualKubelet pods are Ready;
	// - a Node is ready if it has the Ready condition set to True.
	numVirtualNodes := len(virtualNodes)
	expectedNodes := expectedNodes(virtualNodes)
	readyVirtualNodes := 0
	readyNodes := 0

	for i := range virtualNodes {
		// Check if all the VirtualKubelet pods are ready.
		vkPods, err := getters.ListVirtualKubeletPodsFromVirtualNode(ctx, r.Client, &virtualNodes[i])
		if err != nil {
			klog.Errorf("an error occurred while listing VirtualKubelet pods for VirtualNode %q: %s", virtualNodes[i].GetName(), err)
			return err
		}
		if allPodsReady(vkPods.Items) {
			readyVirtualNodes++
		}

		// Check if the Node corresponding to the VirtualNode is ready.
		node, err := getters.GetNodeFromVirtualNode(ctx, r.Client, &virtualNodes[i])
		switch {
		case errors.IsNotFound(err):
			continue
		case err != nil:
			klog.Errorf("an error occurred while getting Node from VirtualNode %q: %s", virtualNodes[i].GetName(), err)
			return err
		default:
			if utils.IsNodeReady(node) {
				readyNodes++
			}
		}
	}

	// Update the Offloading conditions based on the number of ready VirtualNodes and Nodes.

	switch readyVirtualNodes {
	case numVirtualNodes:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1alpha1.OffloadingVirtualNodeStatusCondition, liqov1alpha1.ConditionStatusReady,
			virtualNodesReadyReason, virtualNodesReadyMessage)
	case 0:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1alpha1.OffloadingVirtualNodeStatusCondition, liqov1alpha1.ConditionStatusNotReady,
			virtualNodesNotReadyReason, virtualNodesNotReadyMessage)
	default:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1alpha1.OffloadingVirtualNodeStatusCondition, liqov1alpha1.ConditionStatusSomeNotReady,
			virtualNodesSomeNotReadyReason, virtualNodesSomeNotReadyMessage)
	}

	switch readyNodes {
	case expectedNodes:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1alpha1.OffloadingNodeStatusCondition, liqov1alpha1.ConditionStatusReady,
			nodesReadyReason, nodesReadyMessage)
	case 0:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1alpha1.OffloadingNodeStatusCondition, liqov1alpha1.ConditionStatusNotReady,
			nodesNotReadyReason, nodesNotReadyMessage)
	default:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1alpha1.OffloadingNodeStatusCondition, liqov1alpha1.ConditionStatusSomeNotReady,
			nodesSomeNotReadyReason, nodesSomeNotReadyMessage)
	}

	return nil
}

func clearModule(module *liqov1alpha1.Module) {
	module.Enabled = false
	module.Conditions = nil
}

func expectedNodes(virtualNodes []vkv1alpha1.VirtualNode) int {
	expectedNodes := 0
	for i := range virtualNodes {
		if virtualNodes[i].Spec.CreateNode != nil && *virtualNodes[i].Spec.CreateNode {
			expectedNodes++
		}
	}
	return expectedNodes
}

func allPodsReady(pods []corev1.Pod) bool {
	for i := range pods {
		if ready, _ := pod.IsPodReady(&pods[i]); !ready {
			return false
		}
	}
	return true
}
