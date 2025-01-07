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
)

// ipAcquire acquires an IP, eventually remapped if conflicts are found.
func (lipam *LiqoIPAM) ipAcquire(prefix netip.Prefix) (*netip.Addr, error) {
	result, err := lipam.IpamCore.IPAcquire(prefix)
	if err != nil {
		return nil, fmt.Errorf("error reserving IP in network %q: %w", prefix.String(), err)
	}
	if result == nil {
		return nil, fmt.Errorf("failed to reserve IP in network %q", prefix.String())
	}

	klog.Infof("Acquired IP %q (network %q)", result.String(), prefix.String())

	if lipam.opts.GraphvizEnabled {
		return result, lipam.IpamCore.ToGraphviz()
	}
	return result, nil
}

// acquireIpWithAddress acquires an IP with a specific address.
func (lipam *LiqoIPAM) ipAcquireWithAddr(addr netip.Addr, prefix netip.Prefix) error {
	result, err := lipam.IpamCore.IPAcquireWithAddr(prefix, addr)
	if err != nil {
		return fmt.Errorf("error reserving IP %q in network %q: %w", addr.String(), prefix.String(), err)
	}
	if result == nil {
		return fmt.Errorf("failed to reserve IP %q in network %q", addr.String(), prefix.Addr())
	}

	klog.Infof("Acquired specific IP %q (%q)", result.String(), prefix.String())
	if lipam.opts.GraphvizEnabled {
		return lipam.IpamCore.ToGraphviz()
	}
	return nil
}

// ipRelease frees an IP, removing it from the cache.
func (lipam *LiqoIPAM) ipRelease(addr netip.Addr, prefix netip.Prefix, gracePeriod time.Duration) error {
	result, err := lipam.IpamCore.IPRelease(prefix, addr, gracePeriod)
	if err != nil {
		return fmt.Errorf("error freeing IP %q (network %q): %w", addr.String(), prefix.String(), err)
	}
	if result == nil {
		klog.Infof("IP %q (network %q) already freed or grace period not over", addr.String(), prefix.String())
		return nil
	}
	klog.Infof("Freed IP %q (network %q)", addr.String(), prefix.String())

	if lipam.opts.GraphvizEnabled {
		return lipam.IpamCore.ToGraphviz()
	}
	return nil
}

// ipIsAvailable checks if an IP is available.
func (lipam *LiqoIPAM) ipIsAvailable(addr netip.Addr, prefix netip.Prefix) (bool, error) {
	allocated, err := lipam.IpamCore.IPIsAllocated(prefix, addr)
	return !allocated, err
}

func (lipam *LiqoIPAM) listIPsOnCluster(ctx context.Context) (map[netip.Addr]netip.Prefix, error) {
	result := make(map[netip.Addr]netip.Prefix)
	var ipList ipamv1alpha1.IPList
	if err := lipam.Client.List(ctx, &ipList); err != nil {
		return nil, err
	}

	for i := range ipList.Items {
		ip := &ipList.Items[i]

		deleting := !ip.GetDeletionTimestamp().IsZero()
		if deleting {
			continue
		}

		address := ip.Status.IP.String()
		if address == "" {
			klog.Warningf("IP %q has no address", ip.Name)
			continue
		}

		cidr := ip.Status.CIDR.String()
		if cidr == "" {
			klog.Warningf("IP %q has no CIDR", ip.Name)
			continue
		}

		addr, err := netip.ParseAddr(address)
		if err != nil {
			return nil, fmt.Errorf("failed to parse IP %q: %w", address, err)
		}

		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CIDR %q: %w", cidr, err)
		}

		result[addr] = prefix
	}

	return result, nil
}
