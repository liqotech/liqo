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

package ipam

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	klog "k8s.io/klog/v2"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
)

// networkAcquire acquires a network, eventually remapped if conflicts are found.
func (lipam *LiqoIPAM) networkAcquire(prefix netip.Prefix) (*netip.Prefix, error) {
	result := lipam.IpamCore.NetworkAcquireWithPrefix(prefix)
	if result == nil {
		result = lipam.IpamCore.NetworkAcquire(prefix.Bits())
		if result == nil {
			return nil, fmt.Errorf("failed to reserve network %q", prefix.String())
		}
	}

	klog.Infof("Acquired network %q -> %q", prefix.String(), result.String())

	if lipam.opts.GraphvizEnabled {
		return result, lipam.IpamCore.ToGraphviz()
	}
	return result, nil
}

// networkAcquireSpecific acquires a network with a specific prefix.
// If the network is already allocated, it returns an error.
func (lipam *LiqoIPAM) networkAcquireSpecific(prefix netip.Prefix) (*netip.Prefix, error) {
	result := lipam.IpamCore.NetworkAcquireWithPrefix(prefix)
	if result == nil {
		return nil, fmt.Errorf("failed to reserve specific network %q", prefix.String())
	}

	klog.Infof("Acquired specific network %q -> %q", prefix.String(), result.String())

	if lipam.opts.GraphvizEnabled {
		return result, lipam.IpamCore.ToGraphviz()
	}
	return result, nil
}

func (lipam *LiqoIPAM) acquirePreallocatedIPs(prefix netip.Prefix, preallocated uint32) error {
	// Check if the network can allocate all preallocated IPs.
	if prefix.Bits() < int(preallocated) {
		return fmt.Errorf("network %s can not preallocate %d IPs (insufficient size)", prefix.String(), preallocated)
	}

	// Range over the first N IPs of the network, where N is the number of preallocated IPs.
	for addr := range ipamutils.FirstNIPsFromPrefix(prefix, preallocated) {
		available, err := lipam.ipIsAvailable(addr, prefix)
		if err != nil {
			return err
		}
		if available {
			if err := lipam.ipAcquireWithAddr(addr, prefix); err != nil {
				return err
			}
		}
		// Else, the IP is already reserved. Do nothing.
	}

	return nil
}

// networkRelease frees a network, removing it from the cache.
func (lipam *LiqoIPAM) networkRelease(prefix netip.Prefix, gracePeriod time.Duration) error {
	result := lipam.IpamCore.NetworkRelease(prefix, gracePeriod)
	if result == nil {
		klog.Infof("Network %q already freed or grace period not over", prefix.String())
		return nil
	}
	klog.Infof("Freed network %q", prefix.String())

	if lipam.opts.GraphvizEnabled {
		return lipam.IpamCore.ToGraphviz()
	}
	return nil
}

// networkIsAvailable checks if a network is available.
func (lipam *LiqoIPAM) networkIsAvailable(prefix netip.Prefix) bool {
	return lipam.IpamCore.NetworkIsAvailable(prefix)
}

type prefixDetails struct {
	preallocated uint32
}

func (lipam *LiqoIPAM) listNetworksOnCluster(ctx context.Context) (map[netip.Prefix]prefixDetails, error) {
	result := make(map[netip.Prefix]prefixDetails)
	var networks ipamv1alpha1.NetworkList
	if err := lipam.Client.List(ctx, &networks); err != nil {
		return nil, err
	}

	for i := range networks.Items {
		net := &networks.Items[i]

		deleting := !net.GetDeletionTimestamp().IsZero()
		if deleting {
			continue
		}

		cidr := net.Status.CIDR.String()
		if cidr == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CIDR %q: %w", cidr, err)
		}

		result[prefix] = prefixDetails{preallocated: net.Spec.PreAllocated}
	}

	return result, nil
}

// isInPool checks if a prefix is contained in the prefixes pool used by the ipam as roots.
func (lipam *LiqoIPAM) isInPool(prefix netip.Prefix) bool {
	return lipam.IpamCore.IsPrefixInRoots(prefix)
}
