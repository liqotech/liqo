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

package consts

// Constants used to name and identify controllers.
// Controller-runtime requires that each controller has a unique name within the container.
// This name is used, for example, to identify a controller univocally in the logs
// and must be a prometheus compatible name (underscores and alphanumeric characters only).
// As a convention to avoid conflicts, we use the name of the reconciled resource (lowercase version of their kind),
// and, if already used, we add a recognizable identifier, separated by the underscore character.
// To catch duplicated names, we name the constant as its value (in CamelCase and stripping the separator character),
// with the prefix "Ctrl".
const (
	// Core.
	CtrlForeignCluster      = "foreigncluster"
	CtrlSecretCRDReplicator = "secret_crdreplicator" //nolint:gosec // not a credential
	CtrlSecretWebhook       = "secret_webhook"

	// Networking.
	CtrlConfigurationExternal  = "configuration_external"
	CtrlConfigurationInternal  = "configuration_internal"
	CtrlConfigurationRemapping = "configuration_remapping"
	CtrlConfigurationRoute     = "configuration_route"
	CtrlConnection             = "connection"
	CtrlFirewallConfiguration  = "firewallconfiguration"
	CtrlGatewayClientExternal  = "gatewayclient_external"
	CtrlGatewayClientInternal  = "gatewayclient_internal"
	CtrlGatewayServerExternal  = "gatewayserver_external"
	CtrlGatewayServerInternal  = "gatewayserver_internal"
	CtrlInternalFabricCM       = "internalfabric_cm"
	CtrlInternalFabricFabric   = "internalfabric_fabric"
	CtrlInternalNodeGeneve     = "internalnode_geneve"
	CtrlInternalNodeRoute      = "internalnode_route"
	CtrlIP                     = "ip"
	CtrlIPRemapping            = "ip_remapping"
	CtrlNetwork                = "network"
	CtrlNode                   = "node"
	CtrlPodGateway             = "pod_gateway"
	CtrlPodGwMasq              = "pod_gw_masq"
	CtrlPodInternalNet         = "pod_internalnet"
	CtrlPublicKey              = "publickey"
	CtrlRouteConfiguration     = "routeconfiguration"
	CtrlWGGatewayClient        = "wggatewayclient"
	CtrlWGGatewayServer        = "wggatewayserver"

	// Authentication.
	CtrlIdentity            = "identity"
	CtrlIdentityCreator     = "identity_creator"
	CtrlRenewLocal          = "renew_local"
	CtrlRenewRemote         = "renew_remote"
	CtrlSecretNonceCreator  = "secret_noncecreator"
	CtrlSecretNonceSigner   = "secret_noncesigner"
	CtrlResourceSliceLocal  = "resourceslice_local"
	CtrlResourceSliceRemote = "resourceslice_remote"
	CtrlTenant              = "tenant"

	// Offloading.
	CtrlNamespaceMap        = "namespacemap"
	CtrlNamespaceOffloading = "namespaceoffloading"
	CtrlNodeFailure         = "node_failure"
	CtrlPodStatus           = "pod_status"
	CtrlShadowEndpointSlice = "shadowendpointslice"
	CtrlShadowPod           = "shadowpod"
	CtrlVirtualNode         = "virtualnode"

	// Cross modules.
	CtrlResourceSliceQuotaCreator = "resourceslice_quotacreator"
	CtrlResourceSliceVNCreator    = "resourceslice_vncreator"
	CtrlPodIPMapping              = "pod_ipmapping"
	CtrlConfigurationIPMapping    = "configuration_ipmapping"
)
