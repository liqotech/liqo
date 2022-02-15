// Copyright 2019-2022 The Liqo Authors
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
	// NetworkManagerIpamPort is the port used by IPAM gRPCs.
	NetworkManagerIpamPort = 6000
	// NetworkManagerServiceName is the service name for IPAM gRPCs.
	NetworkManagerServiceName = "liqo-network-manager"
	// DefaultCIDRValue is the default value for a string that contains a CIDR.
	DefaultCIDRValue = "None"
	// NatMappingKind is the constant representing
	// the value of the Kind field of all NatMapping resources.
	NatMappingKind = "NatMapping"
	// RoutingTableID used to identify the custom routing table used
	// to configure the routes on the k8s nodes by route operator.
	RoutingTableID = 18952
	// OverlayNetPrefix prefix of the subnet used for the overlay network.
	// The last three octets of the IP addresses used for the vxlan devices,
	// are taken from the IPs of the nodes. In next PRs it will be introduced
	// new method to allocate non conflict IPs from a user defined subnet for
	// the overlay interfaces.
	OverlayNetPrefix = "240"
	// LiqoRouteOperatorName holds the name of the route operator.
	LiqoRouteOperatorName = "liqo-route"
	// LiqoGatewayOperatorName name of the operator.
	LiqoGatewayOperatorName = "liqo-gateway"
	// LiqoNetworkManagerName name of the operator.
	LiqoNetworkManagerName = "liqo-network-manager"
	// GatewayLeaderElectionID used as name for the lease.coordination.k8s.io resource.
	GatewayLeaderElectionID = "1d5hml1.gateway.net.liqo.io"
	// GatewayNetnsName name of the custom network namespace used by liqo-gateway.
	GatewayNetnsName = "liqo-netns"
	// HostVethName name of the veth device living in the host network namespace,
	// on the node where liqo-gateway is running.
	HostVethName = "liqo.host"
	// HostVethIPAddr is used as next hop when configuring routes for traffic coming
	// from the gateway namespace. A trick to prevent arp requests for the traffic going
	// through the veth pair.
	HostVethIPAddr = "169.254.100.2"
	// GatewayVethName nome of the veth device living in the custom network namespace
	// created by liqo-gateway.
	GatewayVethName = "liqo.gateway"
	// GatewayVethIPAddr is used as next hop when configuring routes for traffic coming
	// from the host namespace. A trick to prevent arp requests for the traffic going
	// through the veth pair.
	GatewayVethIPAddr = "169.254.100.1"
	// VxlanDeviceName name used for the vxlan devices created on each node by the instances
	// of liqo-route.
	VxlanDeviceName = "liqo.vxlan"
	// OverlayNetworkPrefix prefix used for the overlay network.
	OverlayNetworkPrefix = "240"
	// OverlayNetworkMask size of the overlay network.
	OverlayNetworkMask = "/8"
	// PodCIDR is a field of the TunnelEndpoint resource.
	PodCIDR = "PodCIDR"
	// ExternalCIDR is a field of the TunnelEndpoint resource.
	ExternalCIDR = "ExternalCIDR"
	// LocalPodCIDR is a field of the TunnelEndpoint resource.
	LocalPodCIDR = "LocalPodCIDR"
	// LocalExternalCIDR is a field of the TunnelEndpoint resource.
	LocalExternalCIDR = "LocalExternalCIDR"
	// LocalNATPodCIDR is a field of the TunnelEndpoint resource.
	LocalNATPodCIDR = "LocalNATPodCIDR"
	// LocalNATExternalCIDR is a field of the TunnelEndpoint resource.
	LocalNATExternalCIDR = "LocalNATExternalCIDR"
	// RemoteNATPodCIDR is a field of the TunnelEndpoint resource.
	RemoteNATPodCIDR = "RemoteNATPodCIDR"
	// RemoteNATExternalCIDR is a field of the TunnelEndpoint resource.
	RemoteNATExternalCIDR = "RemoteNATExternalCIDR"
	// FinalizersSuffix suffix used by the network operators to create the finalizers added to k8s resources.
	FinalizersSuffix = "net.liqo.io"
	// UDPMinPort min value for a udp port.
	UDPMinPort = 1
	// UDPMaxPort max value for a udp port.
	UDPMaxPort = 65535
	// DefaultMTU default value for the mtu used in the network interfaces managed by the network operators.
	// Used by:
	//  - the route operator for the vxlan interfaces;
	//  - the gateway operator for vpn tunnel and veth pair between host network namespace and custom network namespace.
	DefaultMTU = 1440
	// GatewayListeningPort port used by the vpn tunnel.
	GatewayListeningPort = 5871

	// **** Liqo Gateway Service ****.

	// GatewayServiceAnnotationKey used to annotate the Gateway service with the IP of the node where the
	// active gateway is running.
	GatewayServiceAnnotationKey = "net.liqo.io/gatewayNodeIP"
	// NetworkConfigNamePrefix prefix used to generate the names of the networkconfigs.
	NetworkConfigNamePrefix = "net-config-"
)
