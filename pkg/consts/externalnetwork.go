// Copyright 2019-2023 The Liqo Authors
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

const (
	// ExternalNetworkLabel is the label added to all components that belong to the external network.
	ExternalNetworkLabel = "networking.liqo.io/external-network"
	// ExternalNetworkLabelValue is the value of the label added to components that belong to the external network.
	ExternalNetworkLabelValue = "true"

	// GatewayResourceLabel is the label added to a gateway resource.
	GatewayResourceLabel = "networking.liqo.io/gateway-resource"
	// GatewayResourceLabelValue is the value of the label added to a gateway resource.
	GatewayResourceLabelValue = "true"

	// GatewayTypeServer indicates a Gateway of type server.
	GatewayTypeServer = "server"
	// GatewayTypeClient indicates a Gateway of type client.
	GatewayTypeClient = "client"

	// PrivateKeyField is the data field of the secrets containing private keys.
	PrivateKeyField = "privateKey"
	// PublicKeyField is the data field of the secrets containing public keys.
	PublicKeyField = "publicKey"

	// ClusterRoleBindingFinalizer is the finalizer added ti the owner when a ClusterRoleBinding is created.
	ClusterRoleBindingFinalizer = "networking.liqo.io/clusterrolebinding"
	// GatewayNameLabel is the label added to a resource to identify the Gateway it belongs to.
	GatewayNameLabel = "networking.liqo.io/gateway-name"
	// GatewayNamespaceLabel is the label added to a resource to identify the namespace of the Gateway it belongs to.
	GatewayNamespaceLabel = "networking.liqo.io/gateway-namespace"
)
