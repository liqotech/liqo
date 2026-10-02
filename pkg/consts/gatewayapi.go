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

package consts

const (
	// GatewayControllerName is the controller name of the virtual GatewayClass, whose Gateways are reflected to the remote clusters.
	GatewayControllerName = "liqo.io/gateway-controller"

	// RemoteGatewayModeAnnotation is the annotation of the Gateways of the virtual class selecting how they are mapped
	// to the remote clusters. By default, they are reflected as new Gateways, using the GatewayClass offered by the remote cluster.
	RemoteGatewayModeAnnotation = "liqo.io/remote-gateway-mode"
	// RemoteGatewayModeShared is the value of the RemoteGatewayModeAnnotation mapping the Gateway to the shared Gateway
	// offered by the remote cluster: the Gateway is not reflected, and its routes are attached to the shared one.
	RemoteGatewayModeShared = "shared"

	// SharedGatewayAddressesAnnotation is the annotation of the reflected routes attached to a shared Gateway, which reports
	// the addresses of the shared Gateway (JSON-encoded, as in the Gateway status). It is set by the cluster offering the
	// shared Gateway, since the consumer cluster is not allowed to access it, and it is used to report the addresses
	// in the status of the local Gateways mapped to the shared one.
	SharedGatewayAddressesAnnotation = "liqo.io/shared-gateway-addresses"
)
