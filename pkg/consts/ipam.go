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

package consts

// NetworkType indicates the type of Network.
type NetworkType string

const (
	// IpamPort is the port used by the IPAM gRPC server.
	IpamPort = 50051

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

	// DefaultCIDRValue is the default value for a string that contains a CIDR.
	DefaultCIDRValue = "None"
)
