// Copyright 2019-2021 The Liqo Authors
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
	// NatMappingResourceLabelKey is the constant representing
	// the key of the label assigned to all NatMapping resources.
	NatMappingResourceLabelKey = "net.liqo.io/natmapping"
	// NatMappingResourceLabelValue is the constant representing
	// the value of the label assigned to all NatMapping resources.
	NatMappingResourceLabelValue = "true"
	// IpamStorageResourceLabelKey is the constant representing
	// the key of the label assigned to all IpamStorage resources.
	IpamStorageResourceLabelKey = "net.liqo.io/ipamstorage"
	// IpamStorageResourceLabelValue is the constant representing
	// the value of the label assigned to all IpamStorage resources.
	IpamStorageResourceLabelValue = "true"
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
	// GatewayVethName nome of the veth device living in the custom network namespace
	// created by liqo-gateway.
	GatewayVethName = "liqo.gateway"
	// GatewayVethIPAddr ip address configured on gateway veth device. It is link local
	// IP address. No traffic leaving the custom network namespace has as source IP this
	// address.
	GatewayVethIPAddr = "169.254.100.1/32"
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
)
