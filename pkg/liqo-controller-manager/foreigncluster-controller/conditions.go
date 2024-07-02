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

const (
	connectionEstablishedReason  = "ConnectionEstablished"
	connectionEstablishedMessage = "The network connection with the foreign cluster is established"

	connectionPendingReason  = "ConnectionPending"
	connectionPendingMessage = "The network connection with the foreign cluster is connecting"

	connectionErrorReason  = "ConnectionError"
	connectionErrorMessage = "The network connection with the foreign cluster is in error"

	gatewaysReadyReason  = "GatewaysReady"
	gatewaysReadyMessage = "All gateway replicas are ready"

	gatewaySomeNotReadyReason  = "GatewaysSomeNotReady"
	gatewaySomeNotReadyMessage = "Some gateway replicas are not ready"

	gatewaysNotReadyReason  = "GatewaysNotReady"
	gatewaysNotReadyMessage = "All gateway replicas are not ready"

	tenantReadyReason  = "TenantReady"
	tenantReadyMessage = "The tenant has been successfully configured"

	tenantNotReadyReason  = "TenantNotReady"
	tenantNotReadyMessage = "The tenant is not correctly configured"

	identityReadyReason  = "IdentityReady"
	identityReadyMessage = "The identity has been successfully configured"

	identityNotReadyReason  = "IdentityNotReady"
	identityNotReadyMessage = "The identity is not correctly configured"

	apiServerReadyReason  = "APIServerReady"
	apiServerReadyMessage = "The foreign cluster API Server is ready"

	apiServerNotReadyReason  = "APIServerNotReady"
	apiServerNotReadyMessage = "The foreign cluster API Server is not ready"

	virtualNodesReadyReason  = "VirtualNodesReady"
	virtualNodesReadyMessage = "All virtual nodes are ready"

	virtualNodesSomeNotReadyReason  = "VirtualNodesSomeNotReady"
	virtualNodesSomeNotReadyMessage = "Some virtual nodes are not ready"

	virtualNodesNotReadyReason  = "VirtualNodesNotReady"
	virtualNodesNotReadyMessage = "All virtual nodes are not ready"

	nodesReadyReason  = "NodesReady"
	nodesReadyMessage = "All nodes are ready"

	nodesSomeNotReadyReason  = "NodesSomeNotReady"
	nodesSomeNotReadyMessage = "Some nodes are not ready"

	nodesNotReadyReason  = "NodesNotReady"
	nodesNotReadyMessage = "All nodes are not ready"
)
