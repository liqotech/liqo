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

package fabricipam

import (
	"fmt"
	"net"
	"sync"
)

// IPAM is the struct that manages the IPAM.
type IPAM struct {
	network          net.IPNet
	allocatedMutex   sync.Mutex
	allocated        map[string]string // map[name]ip
	allocatedReverse map[string]string // map[ip]name
	lastAllocatedIP  net.IP
}

// newIPAM returns a newIPAM IPAM.
func newIPAM(network string) (*IPAM, error) {
	ip, ipNet, err := net.ParseCIDR(network)
	if err != nil {
		return nil, err
	}

	// allocate first and last ip
	allocated := make(map[string]string)
	allocatedReverse := make(map[string]string)

	allocated[ipNet.IP.String()] = ipNet.IP.String()
	allocatedReverse[ipNet.IP.String()] = ipNet.IP.String()

	o, b := ipNet.Mask.Size()
	nIPs := 1 << uint(b-o)
	lastIP := add(net.ParseIP(ipNet.IP.String()), uint(nIPs-1))

	allocated[lastIP.String()] = lastIP.String()
	allocatedReverse[lastIP.String()] = lastIP.String()

	return &IPAM{
		network:          *ipNet,
		allocated:        allocated,
		allocatedReverse: allocatedReverse,
		lastAllocatedIP:  ip,
	}, nil
}

// configure configures the IPAM with the given name and ip.
// It is used to preallocate IPs already assigned in previous executions.
func (i *IPAM) configure(name, ip string) error {
	i.allocatedMutex.Lock()
	defer i.allocatedMutex.Unlock()
	if _, ok := i.allocated[name]; ok {
		return fmt.Errorf("name %s already configured", name)
	}
	if _, ok := i.allocatedReverse[ip]; ok {
		return fmt.Errorf("ip %s already configured", ip)
	}
	i.allocated[name] = ip
	i.allocatedReverse[ip] = name
	return nil
}

// isIPConfigured returns true if the given IP is already configured.
func (i *IPAM) isIPConfigured(ip string) bool {
	i.allocatedMutex.Lock()
	defer i.allocatedMutex.Unlock()
	_, ok := i.allocatedReverse[ip]
	return ok
}

// Allocate allocates an IP for the given name.
func (i *IPAM) Allocate(name string) (net.IP, error) {
	i.allocatedMutex.Lock()
	defer i.allocatedMutex.Unlock()
	if v, ok := i.allocated[name]; ok {
		return net.ParseIP(v), nil
	}
	for ip := i.lastAllocatedIP; i.network.Contains(ip); inc(ip) {
		if _, ok := i.allocatedReverse[ip.String()]; !ok {
			i.lastAllocatedIP = ip
			i.allocated[name] = ip.String()
			i.allocatedReverse[ip.String()] = name
			return ip, nil
		}
	}
	for ip := i.network.IP.Mask(i.network.Mask); i.network.Contains(ip); inc(ip) {
		if _, ok := i.allocatedReverse[ip.String()]; !ok {
			i.lastAllocatedIP = ip
			i.allocated[name] = ip.String()
			i.allocatedReverse[ip.String()] = name
			return ip, nil
		}
	}
	return nil, fmt.Errorf("no more IP available")
}

func add(ip net.IP, n uint) net.IP {
	ip = ip.To4()
	if ip == nil {
		ip = ip.To16()
	}
	if ip == nil {
		return nil
	}
	for j := len(ip) - 1; j >= 0 && n > 0; j-- {
		tmp := uint(ip[j]) + n
		ip[j] = byte(tmp)
		n = tmp >> 8
	}
	return ip
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
