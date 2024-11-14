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

package ipam

import (
	"fmt"

	"github.com/google/nftables"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// IsAPIServerIP checks if the resource is an IP of type API server.
func IsAPIServerIP(ip *ipamv1alpha1.IP) bool {
	ipType, ok := ip.Labels[consts.IPTypeLabelKey]
	return ok && ipType == consts.IPTypeAPIServer
}

// IsAPIServerProxyIP checks if the resource is an IP of type API server proxy.
func IsAPIServerProxyIP(ip *ipamv1alpha1.IP) bool {
	ipType, ok := ip.Labels[consts.IPTypeLabelKey]
	return ok && ipType == consts.IPTypeAPIServerProxy
}

// GetRemappedIP returns the remapped IP of the given IP resource.
func GetRemappedIP(ip *ipamv1alpha1.IP) networkingv1beta1.IP {
	return ip.Status.IP
}

// GetUnknownSourceIP returns the IP address used to map unknown sources.
func GetUnknownSourceIP(extCIDR string) (string, error) {
	if extCIDR == "" {
		return "", fmt.Errorf("ExternalCIDR not set")
	}
	firstExtCIDRip, _, err := nftables.NetFirstAndLastIP(extCIDR)
	if err != nil {
		return "", fmt.Errorf("cannot get first IP of ExternalCIDR")
	}
	return firstExtCIDRip.String(), nil
}
