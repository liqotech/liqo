// Copyright 2019-2025 The Liqo Authors
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

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/gateway/forge"
	"github.com/liqotech/liqo/pkg/utils"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/pod"
)

type statusException struct {
	liqov1beta1.ConditionStatusType
	Reason  string
	Message string
}

func (r *ForeignClusterReconciler) clearStatusExceptConditions(foreignCluster *liqov1beta1.ForeignCluster) {
	foreignCluster.Status = liqov1beta1.ForeignClusterStatus{
		Role: liqov1beta1.UnknownRole,
		Modules: liqov1beta1.Modules{
			Networking: liqov1beta1.Module{
				Enabled:    false,
				Conditions: foreignCluster.Status.Modules.Networking.Conditions,
			},
			Authentication: liqov1beta1.Module{
				Enabled:    false,
				Conditions: foreignCluster.Status.Modules.Authentication.Conditions,
			},
			Offloading: liqov1beta1.Module{
				Enabled:    false,
				Conditions: foreignCluster.Status.Modules.Offloading.Conditions,
			},
		},
		Conditions: foreignCluster.Status.Conditions,
	}
}

func (r *ForeignClusterReconciler) handleConnectionStatus(ctx context.Context,
	fc *liqov1beta1.ForeignCluster, statusExceptions map[liqov1beta1.ConditionType]statusException) error {
	clusterID := fc.Spec.ClusterID

	connection, err := getters.GetConnectionByClusterID(ctx, r.Client, string(clusterID))
	switch {
	case errors.IsNotFound(err):
		klog.V(6).Infof("Connection resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Networking, liqov1beta1.NetworkConnectionStatusCondition)
		statusExceptions[liqov1beta1.NetworkConnectionStatusCondition] = statusException{
			ConditionStatusType: liqov1beta1.ConditionStatusNotReady,
			Reason:              connectionMissingReason,
			Message:             connectionMissingMessage,
		}
	case err != nil:
		klog.Errorf("an error occurred while getting the Connection resource for the ForeignCluster %q: %s", clusterID, err)
		return err
	default:
		fcutils.EnableModuleNetworking(fc)
		switch connection.Status.Value {
		case networkingv1beta1.Connected:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1beta1.NetworkConnectionStatusCondition, liqov1beta1.ConditionStatusEstablished,
				connectionEstablishedReason, connectionEstablishedMessage)
		case networkingv1beta1.Connecting:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1beta1.NetworkConnectionStatusCondition, liqov1beta1.ConditionStatusPending,
				connectionPendingReason, connectionPendingMessage)
		default:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1beta1.NetworkConnectionStatusCondition, liqov1beta1.ConditionStatusError,
				connectionErrorReason, connectionErrorMessage)
		}
	}
	return nil
}

func (r *ForeignClusterReconciler) handleGatewaysStatus(ctx context.Context,
	fc *liqov1beta1.ForeignCluster, statusExceptions map[liqov1beta1.ConditionType]statusException) error {
	clusterID := fc.Spec.ClusterID

	gwServer, errServer := getters.GetGatewayServerByClusterID(ctx, r.Client, clusterID, corev1.NamespaceAll)
	gwClient, errClient := getters.GetGatewayClientByClusterID(ctx, r.Client, clusterID, corev1.NamespaceAll)

	if errors.IsNotFound(errServer) && errors.IsNotFound(errClient) {
		klog.V(6).Infof("Both GatewayServer and GatewayClient resources not found for ForeignCluster %q", clusterID)
		statusExceptions[liqov1beta1.NetworkGatewayPresenceCondition] = statusException{
			ConditionStatusType: liqov1beta1.ConditionStatusNotReady,
			Reason:              gatewayMissingReason,
			Message:             gatewayMissingMessage,
		}
	} else {
		statusExceptions[liqov1beta1.NetworkGatewayPresenceCondition] = statusException{
			ConditionStatusType: liqov1beta1.ConditionStatusReady,
			Reason:              gatewayPresentReason,
			Message:             gatewayPresentMessage,
		}
	}

	switch {
	case errors.IsNotFound(errServer):
		klog.V(6).Infof("GatewayServer resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Networking, liqov1beta1.NetworkGatewayServerStatusCondition)
	case errServer != nil:
		klog.Errorf("an error occurred while getting the GatewayServer resource for the ForeignCluster %q: %s", clusterID, errServer)
		return errServer
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
				liqov1beta1.NetworkGatewayServerStatusCondition, liqov1beta1.ConditionStatusNotReady,
				gatewaysNotReadyReason, gatewaysNotReadyMessage)
		case gwDeployment.Status.UnavailableReplicas > 0:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1beta1.NetworkGatewayServerStatusCondition, liqov1beta1.ConditionStatusSomeNotReady,
				gatewaySomeNotReadyReason, gatewaySomeNotReadyMessage)
		default:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1beta1.NetworkGatewayServerStatusCondition, liqov1beta1.ConditionStatusReady,
				gatewaysReadyReason, gatewaysReadyMessage)
		}
	}

	switch {
	case errors.IsNotFound(errClient):
		klog.V(6).Infof("GatewayClient resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Networking, liqov1beta1.NetworkGatewayClientStatusCondition)
	case errClient != nil:
		klog.Errorf("an error occurred while getting the GatewayClient resource for the ForeignCluster %q: %s", clusterID, errClient)
		return errClient
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
				liqov1beta1.NetworkGatewayClientStatusCondition, liqov1beta1.ConditionStatusNotReady,
				gatewaysNotReadyReason, gatewaysNotReadyMessage)
		case gwDeployment.Status.UnavailableReplicas > 0:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1beta1.NetworkGatewayClientStatusCondition, liqov1beta1.ConditionStatusSomeNotReady,
				gatewaySomeNotReadyReason, gatewaySomeNotReadyMessage)
		default:
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				liqov1beta1.NetworkGatewayClientStatusCondition, liqov1beta1.ConditionStatusReady,
				gatewaysReadyReason, gatewaysReadyMessage)
		}
	}

	return nil
}

func (r *ForeignClusterReconciler) handleNetworkConfigurationStatus(ctx context.Context,
	fc *liqov1beta1.ForeignCluster, statusExceptions map[liqov1beta1.ConditionType]statusException) error {
	clusterID := fc.Spec.ClusterID
	_, err := getters.GetConfigurationByClusterID(ctx, r.Client, clusterID, corev1.NamespaceAll)
	switch {
	case errors.IsNotFound(err):
		klog.V(6).Infof("Configuration resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Networking, liqov1beta1.NetworkConfigurationStatusCondition)
		statusExceptions[liqov1beta1.NetworkConfigurationStatusCondition] = statusException{
			ConditionStatusType: liqov1beta1.ConditionStatusNotReady,
			Reason:              networkConfigurationMissingReason,
			Message:             networkConfigurationMissingMessage,
		}
	case err != nil:
		klog.Errorf("an error occurred while getting the Configuration resource for the ForeignCluster %q: %s", clusterID, err)
		return err
	default:
		fcutils.EnableModuleNetworking(fc)
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
			liqov1beta1.NetworkConfigurationStatusCondition, liqov1beta1.ConditionStatusReady,
			networkConfigurationPresenceReason, networkConfigurationPresenceMessage)
	}
	return nil
}

func (r *ForeignClusterReconciler) handleNetworkingModuleStatus(ctx context.Context,
	fc *liqov1beta1.ForeignCluster, moduleEnabled bool) error {
	if !moduleEnabled {
		clearModule(&fc.Status.Modules.Networking)
		return nil
	}

	statusExceptions := map[liqov1beta1.ConditionType]statusException{}

	if err := r.handleNetworkConfigurationStatus(ctx, fc, statusExceptions); err != nil {
		return err
	}

	if err := r.handleGatewaysStatus(ctx, fc, statusExceptions); err != nil {
		return err
	}

	if err := r.handleConnectionStatus(ctx, fc, statusExceptions); err != nil {
		return err
	}

	// Write the exception in the status if the module is enabled
	if fc.Status.Modules.Networking.Enabled {
		for condition, condDescription := range statusExceptions {
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Networking,
				condition, condDescription.ConditionStatusType,
				condDescription.Reason, condDescription.Message)
		}
	}

	return nil
}

func (r *ForeignClusterReconciler) handleAuthenticationModuleStatus(ctx context.Context,
	fc *liqov1beta1.ForeignCluster, moduleEnabled bool, consumer, provider *bool) error {
	if !moduleEnabled {
		clearModule(&fc.Status.Modules.Authentication)
		return nil
	}

	clusterID := fc.Spec.ClusterID

	// Check if a Tenant resource for this cluser exists.
	tenant, err := getters.GetTenantByClusterID(ctx, r.Client, clusterID, corev1.NamespaceAll)
	switch {
	case errors.IsNotFound(err):
		klog.V(6).Infof("Tenant resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Authentication, liqov1beta1.AuthTenantStatusCondition)
	case err != nil:
		klog.Errorf("an error occurred while getting the Tenant resource for the ForeignCluster %q: %s", clusterID, err)
		return err
	default:
		*consumer = true
		fcutils.EnableModuleAuthentication(fc)
		if tenant.Status.TenantNamespace != "" {
			fc.Status.TenantNamespace.Local = tenant.Status.TenantNamespace
		}

		// Define the status of the authentication module based on whether the keys exchange has been performed.
		expectKeysExchange := authv1beta1.GetAuthzPolicyValue(tenant.Spec.AuthzPolicy) != authv1beta1.TolerateNoHandshake
		if expectKeysExchange && tenant.Status.AuthParams == nil || tenant.Status.TenantNamespace == "" {
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Authentication,
				liqov1beta1.AuthTenantStatusCondition, liqov1beta1.ConditionStatusNotReady,
				tenantNotReadyReason, tenantNotReadyMessage)
		} else {
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Authentication,
				liqov1beta1.AuthTenantStatusCondition, liqov1beta1.ConditionStatusReady,
				tenantReadyReason, tenantReadyMessage)
		}
	}

	// Check if an Identity resource of type ControlPlane for this cluster exists.
	identity, err := getters.GetControlPlaneIdentityByClusterID(ctx, r.Client, clusterID)
	switch {
	case errors.IsNotFound(err):
		klog.V(6).Infof("Identity resource not found for ForeignCluster %q", clusterID)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Authentication, liqov1beta1.AuthIdentityControlPlaneStatusCondition)
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
				liqov1beta1.AuthIdentityControlPlaneStatusCondition, liqov1beta1.ConditionStatusNotReady,
				identityNotReadyReason, identityNotReadyMessage)
		} else {
			fcutils.EnsureModuleCondition(&fc.Status.Modules.Authentication,
				liqov1beta1.AuthIdentityControlPlaneStatusCondition, liqov1beta1.ConditionStatusReady,
				identityReadyReason, identityReadyMessage)
		}
	}

	return nil
}

func (r *ForeignClusterReconciler) handleOffloadingModuleStatus(ctx context.Context,
	fc *liqov1beta1.ForeignCluster, moduleEnabled bool, provider *bool) error {
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
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Offloading, liqov1beta1.OffloadingVirtualNodeStatusCondition)
		fcutils.DeleteModuleCondition(&fc.Status.Modules.Offloading, liqov1beta1.OffloadingNodeStatusCondition)
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
			liqov1beta1.OffloadingVirtualNodeStatusCondition, liqov1beta1.ConditionStatusReady,
			virtualNodesReadyReason, virtualNodesReadyMessage)
	case 0:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1beta1.OffloadingVirtualNodeStatusCondition, liqov1beta1.ConditionStatusNotReady,
			virtualNodesNotReadyReason, virtualNodesNotReadyMessage)
	default:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1beta1.OffloadingVirtualNodeStatusCondition, liqov1beta1.ConditionStatusSomeNotReady,
			virtualNodesSomeNotReadyReason, virtualNodesSomeNotReadyMessage)
	}

	switch readyNodes {
	case expectedNodes:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1beta1.OffloadingNodeStatusCondition, liqov1beta1.ConditionStatusReady,
			nodesReadyReason, nodesReadyMessage)
	case 0:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1beta1.OffloadingNodeStatusCondition, liqov1beta1.ConditionStatusNotReady,
			nodesNotReadyReason, nodesNotReadyMessage)
	default:
		fcutils.EnsureModuleCondition(&fc.Status.Modules.Offloading,
			liqov1beta1.OffloadingNodeStatusCondition, liqov1beta1.ConditionStatusSomeNotReady,
			nodesSomeNotReadyReason, nodesSomeNotReadyMessage)
	}

	return nil
}

func clearModule(module *liqov1beta1.Module) {
	module.Enabled = false
	module.Conditions = nil
}

func expectedNodes(virtualNodes []offloadingv1beta1.VirtualNode) int {
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
