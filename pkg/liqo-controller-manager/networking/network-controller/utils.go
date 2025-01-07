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

package networkctrl

import (
	"context"

	"k8s.io/klog/v2"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/ipam"
)

// getRemappedCIDR returns the remapped CIDR for the given CIDR.
func getRemappedCIDR(ctx context.Context, ipamClient ipam.IPAMClient,
	desiredCIDR networkingv1beta1.CIDR, immutable bool, preallocated uint32) (networkingv1beta1.CIDR, error) {
	switch ipamClient.(type) {
	case nil:
		// IPAM is not enabled, use original CIDR from spec
		return desiredCIDR, nil
	default:
		// interact with the IPAM to retrieve the correct mapping.
		response, err := ipamClient.NetworkAcquire(ctx, &ipam.NetworkAcquireRequest{
			Cidr:         desiredCIDR.String(),
			Immutable:    immutable,
			PreAllocated: preallocated,
		})
		if err != nil {
			klog.Errorf("IPAM: error while mapping network CIDR %s: %v", desiredCIDR, err)
			return "", err
		}
		klog.Infof("IPAM: mapped network CIDR %s to %s", desiredCIDR, response.Cidr)
		return networkingv1beta1.CIDR(response.Cidr), nil
	}
}

// deleteRemappedCIDR unmaps the given CIDR.
func deleteRemappedCIDR(ctx context.Context, ipamClient ipam.IPAMClient,
	remappedCIDR networkingv1beta1.CIDR) error {
	switch ipamClient.(type) {
	case nil:
		// If the IPAM is not enabled we do not need to free the network CIDR.
		return nil
	default:
		// Interact with the IPAM to free the network CIDR.
		_, err := ipamClient.NetworkRelease(ctx, &ipam.NetworkReleaseRequest{
			Cidr: remappedCIDR.String(),
		})
		if err != nil {
			klog.Errorf("IPAM: error while unmapping CIDR %s: %v", remappedCIDR, err)
			return err
		}
		klog.Infof("IPAM: unmapped CIDR %s", remappedCIDR)
		return nil
	}
}
