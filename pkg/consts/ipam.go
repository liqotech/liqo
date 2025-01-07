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

import "time"

// NetworkType indicates the type of Network.
type NetworkType string

const (
	// IpamPort is the port used by the IPAM gRPC server.
	IpamPort = 6000
	// SyncInterval is the frequency at which the IPAM should periodically sync its status.
	SyncInterval = 2 * time.Minute
	// SyncGracePeriod is the time the IPAM sync routine should wait before performing a deletion.
	SyncGracePeriod = 30 * time.Second
	// NetworkNotRemappedLabelKey is the label key used to mark a Network that does not need CIDR remapping.
	NetworkNotRemappedLabelKey = "ipam.liqo.io/network-not-remapped"
	// NetworkNotRemappedLabelValue is the label value used to mark a Network that does not need CIDR remapping.
	NetworkNotRemappedLabelValue = "true"

	// NetworkTypeLabelKey is the label key used to indicate the type of a Network.
	NetworkTypeLabelKey = "ipam.liqo.io/network-type"
	// NetworkTypePodCIDR is the constant representing a network of type podCIDR.
	NetworkTypePodCIDR NetworkType = "pod-cidr"
	// NetworkTypeServiceCIDR is the constant representing a network of type serviceCIDR.
	NetworkTypeServiceCIDR NetworkType = "service-cidr"
	// NetworkTypeExternalCIDR is the constant representing a network of type externalCIDR.
	NetworkTypeExternalCIDR NetworkType = "external-cidr"
	// NetworkTypeInternalCIDR is the constant representing a network of type internalCIDR.
	NetworkTypeInternalCIDR NetworkType = "internal-cidr"
	// NetworkTypeReserved is the constant representing a network of type reserved subnet.
	NetworkTypeReserved NetworkType = "reserved"

	// IPTypeLabelKey is the label key used to indicate the type of an IP.
	IPTypeLabelKey = "ipam.liqo.io/ip-type"
	// IPTypeAPIServer is the constant representing an IP of type APIServer.
	IPTypeAPIServer = "api-server"
	// IPTypeAPIServerProxy is the constant representing an IP of type APIServerProxy.
	IPTypeAPIServerProxy = "api-server-proxy"

	// NetworkNamespaceLabelKey is the label key used to indicate the namespace of a Network.
	NetworkNamespaceLabelKey = "ipam.liqo.io/network-namespace"
	// NetworkNameLabelKey is the label key used to indicate the name of a Network.
	NetworkNameLabelKey = "ipam.liqo.io/network-name"

	// DefaultCIDRValue is the default value for a string that contains a CIDR.
	DefaultCIDRValue = "None"
)

var (
	// PrivateAddressSpace contains all the ranges for private addresses as defined in RFC1918.
	PrivateAddressSpace = []string{"10.0.0.0/8", "192.168.0.0/16", "172.16.0.0/12"}
)
