// Copyright 2019-2023 The Liqo Authors
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

package resolver

import (
	"context"
	"fmt"
	"net"
	"sync"
)

var (
	resolverFunc = net.DefaultResolver.LookupIPAddr
	resolveCache = sync.Map{}
)

// Resolve resolves the given hostname to an IP address.
// It prefers IPv4 addresses if available, and falls back to IPv6 addresses if not.
// If the hostname was already resolved and it is still valid, it returns the cached IP address.
func Resolve(ctx context.Context, address string) (*net.IPAddr, error) {
	if ip := net.ParseIP(address); ip != nil {
		return &net.IPAddr{IP: ip}, nil
	}

	ipAddrs, err := resolverFunc(ctx, address)
	if err != nil {
		return nil, err
	}

	ipv4List := []*net.IPAddr{}
	ipv6List := []*net.IPAddr{}

	for i := range ipAddrs {
		ip := ipAddrs[i]
		if ipv4 := ip.IP.To4(); ipv4 != nil {
			ipv4List = append(ipv4List, &ip)
		}
		if ipv6 := ip.IP.To16(); ipv6 != nil {
			ipv6List = append(ipv6List, &ip)
		}
	}

	if len(ipv4List) > 0 {
		if ip := lookupCache(address, ipv4List); ip != nil {
			return ip, nil
		}
		resolveCache.Store(address, ipv4List[0])
		return ipv4List[0], nil
	}
	if len(ipv6List) > 0 {
		if ip := lookupCache(address, ipv6List); ip != nil {
			return ip, nil
		}
		resolveCache.Store(address, ipv6List[0])
		return ipv6List[0], nil
	}

	return nil, fmt.Errorf("no IP addresses found for %q", address)
}

func lookupCache(hostname string, resolvedIPs []*net.IPAddr) *net.IPAddr {
	v, found := resolveCache.Load(hostname)
	if !found {
		return nil
	}
	cachedIP := v.(*net.IPAddr)

	for _, resIP := range resolvedIPs {
		if resIP.IP.Equal(cachedIP.IP) {
			return cachedIP
		}
	}

	return nil
}
