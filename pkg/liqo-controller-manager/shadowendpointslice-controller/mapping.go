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

package shadowendpointslicectrl

import (
	"context"
	"net"

	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// MapEndpointsWithConfiguration maps the endpoints of the shadowendpointslice.
func MapEndpointsWithConfiguration(ctx context.Context, cl client.Client,
	clusterID discoveryv1alpha1.ClusterID, endpoints []discoveryv1.Endpoint) error {
	cfg, err := getters.GetConfigurationByClusterID(ctx, cl, clusterID)
	if err != nil {
		return err
	}

	var (
		podnet, podnetMapped, extnet, extnetMapped *net.IPNet
		podNetMaskLen, extNetMaskLen               int
	)

	podNeedsRemap := cfg.Spec.Remote.CIDR.Pod.String() != cfg.Status.Remote.CIDR.Pod.String()
	extNeedsRemap := cfg.Spec.Remote.CIDR.External.String() != cfg.Status.Remote.CIDR.External.String()

	_, podnet, err = net.ParseCIDR(cfg.Spec.Remote.CIDR.Pod.String())
	if err != nil {
		return err
	}
	if podNeedsRemap {
		_, podnetMapped, err = net.ParseCIDR(cfg.Status.Remote.CIDR.Pod.String())
		if err != nil {
			return err
		}
		podNetMaskLen, _ = podnetMapped.Mask.Size()
	}

	_, extnet, err = net.ParseCIDR(cfg.Spec.Remote.CIDR.External.String())
	if err != nil {
		return err
	}
	if extNeedsRemap {
		_, extnetMapped, err = net.ParseCIDR(cfg.Status.Remote.CIDR.External.String())
		if err != nil {
			return err
		}
		extNetMaskLen, _ = extnetMapped.Mask.Size()
	}

	for i := range endpoints {
		for j := range endpoints[i].Addresses {
			addr := endpoints[i].Addresses[j]
			paddr := net.ParseIP(addr)
			if podNeedsRemap && podnet.Contains(paddr) {
				endpoints[i].Addresses[j] = remapMask(paddr, *podnetMapped, podNetMaskLen).String()
			}
			if extNeedsRemap && extnet.Contains(paddr) {
				endpoints[i].Addresses[j] = remapMask(paddr, *extnetMapped, extNetMaskLen).String()
			}
		}
	}

	return nil
}

// remapMask remaps the mask of the address.
// Consider that net.IP is always a slice of 16 bytes (big-endian).
// The mask is a slice of 4 or 16 bytes (big-endian).
func remapMask(addr net.IP, mask net.IPNet, maskLen int) net.IP {
	maskLenBytes := maskLen / 8
	for i := 0; i < maskLenBytes; i++ {
		// i+(len(addr)-len(mask.IP)) allows to start from the rightmost byte of the address.
		// e.g if addr is ipv4 len(addr) = 16, and mask is ipv4 len(mask.IP) = 4, then we start from addr[12].
		addr[i+(len(addr)-len(mask.IP))] = mask.IP[i]
	}
	return addr
}
