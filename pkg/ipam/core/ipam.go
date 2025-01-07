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

package ipamcore

import (
	"fmt"
	"net/netip"
	"time"
)

// Ipam represents the IPAM core structure.
type Ipam struct {
	roots []node
}

// NewIpam creates a new IPAM instance.
func NewIpam(pools []netip.Prefix) (*Ipam, error) {
	if err := checkRoots(pools); err != nil {
		return nil, err
	}

	ipamRoots := make([]node, len(pools))
	for i := range pools {
		ipamRoots[i] = newNode(pools[i])
	}

	ipam := &Ipam{
		roots: ipamRoots,
	}

	return ipam, nil
}

// NetworkAcquire allocates a network of the given size.
// It returns the allocated network or nil if no network is available.
func (ipam *Ipam) NetworkAcquire(size int) *netip.Prefix {
	for i := range ipam.roots {
		if result := allocateNetwork(size, &ipam.roots[i]); result != nil {
			return result
		}
	}
	return nil
}

// NetworkAcquireWithPrefix allocates a network with the given prefix.
// It returns the allocated network or nil if the network is not available.
func (ipam *Ipam) NetworkAcquireWithPrefix(prefix netip.Prefix) *netip.Prefix {
	for i := range ipam.roots {
		if isPrefixChildOf(ipam.roots[i].prefix, prefix) {
			if result := allocateNetworkWithPrefix(prefix, &ipam.roots[i]); result != nil {
				return result
			}
		}
	}
	return nil
}

// NetworkRelease frees the network with the given prefix.
// It returns the freed network or nil if the network is not found.
func (ipam *Ipam) NetworkRelease(prefix netip.Prefix, gracePeriod time.Duration) *netip.Prefix {
	for i := range ipam.roots {
		if isPrefixChildOf(ipam.roots[i].prefix, prefix) {
			if result := networkRelease(prefix, &ipam.roots[i], gracePeriod); result != nil {
				return result
			}
		}
	}
	return nil
}

// ListNetworks returns the list of allocated networks.
func (ipam *Ipam) ListNetworks() []netip.Prefix {
	var networks []netip.Prefix
	for i := range ipam.roots {
		networks = append(networks, listNetworks(&ipam.roots[i])...)
	}
	return networks
}

// NetworkIsAvailable checks if the network with the given prefix is allocated.
// It returns false if the network is allocated or there is no suitable pool, true otherwise.
func (ipam *Ipam) NetworkIsAvailable(prefix netip.Prefix) bool {
	for i := range ipam.roots {
		if isPrefixChildOf(ipam.roots[i].prefix, prefix) {
			return networkIsAvailable(prefix, &ipam.roots[i])
		}
	}
	return false
}

// IPAcquire allocates an IP address from the given prefix.
// It returns the allocated IP address or nil if the IP address is not available.
func (ipam *Ipam) IPAcquire(prefix netip.Prefix) (*netip.Addr, error) {
	node, err := ipam.search(prefix)
	if err != nil {
		return nil, err
	}
	if node != nil {
		return node.ipAcquire(), nil
	}

	return nil, nil
}

// IPAcquireWithAddr allocates the IP address from the given prefix.
// It returns the allocated IP address or nil if the IP address is not available.
func (ipam *Ipam) IPAcquireWithAddr(prefix netip.Prefix, addr netip.Addr) (*netip.Addr, error) {
	if !prefix.Contains(addr) {
		return nil, fmt.Errorf("address %s is not contained in prefix %s", addr, prefix)
	}
	node, err := ipam.search(prefix)
	if err != nil {
		return nil, err
	}
	if node != nil {
		return node.allocateIPWithAddr(addr), nil
	}
	return nil, nil
}

// IPRelease frees the IP address from the given prefix.
// It returns the freed IP address or nil if the IP address is not found.
func (ipam *Ipam) IPRelease(prefix netip.Prefix, addr netip.Addr, gracePeriod time.Duration) (*netip.Addr, error) {
	node, err := ipam.search(prefix)
	if err != nil {
		return nil, err
	}
	if node != nil {
		return node.ipRelease(addr, gracePeriod), nil
	}
	return nil, nil
}

// ListIPs returns the list of allocated IP addresses from the given prefix.
func (ipam *Ipam) ListIPs(prefix netip.Prefix) ([]netip.Addr, error) {
	node, err := ipam.search(prefix)
	if err != nil {
		return nil, err
	}
	if node != nil {
		addrs := make([]netip.Addr, len(node.ips))
		for i := range node.ips {
			addrs[i] = node.ips[i].addr
		}
		return addrs, nil
	}
	return nil, nil
}

// IPIsAllocated checks if the IP address is allocated from the given prefix.
// It returns true if the IP address is allocated, false otherwise.
func (ipam *Ipam) IPIsAllocated(prefix netip.Prefix, addr netip.Addr) (bool, error) {
	node, err := ipam.search(prefix)
	if err != nil {
		return false, err
	}
	if node != nil {
		return node.isAllocatedIP(addr), nil
	}
	return false, nil
}

// IsPrefixInRoots checks if the given prefix is contained in the roots.
// It returns true if the prefix is contained, false otherwise.
func (ipam *Ipam) IsPrefixInRoots(prefix netip.Prefix) bool {
	for i := range ipam.roots {
		if isPrefixChildOf(ipam.roots[i].prefix, prefix) {
			return true
		}
	}
	return false
}

// ToGraphviz generates the Graphviz representation of the IPAM structure.
func (ipam *Ipam) ToGraphviz() error {
	for i := range ipam.roots {
		_ = i
		if err := ipam.roots[i].toGraphviz(); err != nil {
			return fmt.Errorf("failed to generate Graphviz representation: %w", err)
		}
	}
	return nil
}

func (ipam *Ipam) search(prefix netip.Prefix) (*node, error) {
	for i := range ipam.roots {
		if !isPrefixChildOf(ipam.roots[i].prefix, prefix) {
			continue
		}
		if node := search(prefix, &ipam.roots[i]); node != nil {
			return node, nil
		}
		return nil, nil
	}
	return nil, fmt.Errorf("prefix %s not contained in roots", prefix)
}

func checkRoots(roots []netip.Prefix) error {
	for i := range roots {
		if err := checkHostBitsZero(roots[i]); err != nil {
			return err
		}
	}
	return nil
}

// NetworkSetLastUpdateTimestamp sets the last update time of the network with the given prefix.
// This function is for testing purposes only.
func (ipam *Ipam) NetworkSetLastUpdateTimestamp(prefix netip.Prefix, lastUpdateTimestamp time.Time) error {
	node, err := ipam.search(prefix)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("prefix %s not found", prefix)
	}
	node.lastUpdateTimestamp = lastUpdateTimestamp
	return nil
}

// IPSetCreationTimestamp sets the creation timestamp of the IP address with the given address.
// This function is for testing purposes only.
func (ipam *Ipam) IPSetCreationTimestamp(addr netip.Addr, prefix netip.Prefix, creationTimestamp time.Time) error {
	node, err := ipam.search(prefix)
	if err != nil {
		return err
	}
	if node == nil {
		return fmt.Errorf("prefix %s not found", prefix)
	}

	for i := range node.ips {
		if node.ips[i].addr.Compare(addr) == 0 {
			node.ips[i].creationTimestamp = creationTimestamp
			return nil
		}
	}
	return fmt.Errorf("IP address %s not found", addr)
}
