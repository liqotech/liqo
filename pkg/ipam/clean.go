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

	"sigs.k8s.io/controller-runtime/pkg/client"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
)

// CheckAndSanitizeIP checks if the IP resource is coherent with its Network and sanitizes it.
func CheckAndSanitizeIPs(ctx context.Context, cl client.Client) error {
	var IpList ipamv1alpha1.IPList
	var network ipamv1alpha1.Network

	if err := cl.List(ctx, &IpList); err != nil {
		return fmt.Errorf("failed to list IPs: %w", err)
	}
	for _, ip := range IpList.Items {
		if !IsIPReady(ip) {
			// When the IP status has not been fulfilled yet we skip the check
			continue
		}

		if err := cl.Get(ctx, client.ObjectKey{
			Namespace: ip.Spec.NetworkRef.Namespace,
			Name:      ip.Spec.NetworkRef.Name,
		}, &network); err != nil {
			return fmt.Errorf("failed to get IP %q: %w", ip.Name, err)
		}

		ok, err := CheckIPCoherentWithNetwork(ip, network)
		if err != nil {
			return fmt.Errorf("IP %q is not coherent with network %q: %w", ip.Name, network.Name, err)
		}
		if !ok {
			ip.Status.CIDR = ""
			ip.Status.IP = ""
			if err := cl.Status().Update(ctx, &ip, &client.SubResourceUpdateOptions{}); err != nil {
				return fmt.Errorf("failed to update IP %q: %w", ip.Name, err)
			}
		}
	}
	return nil
}

// IsIPReady checks if the IP resource status has been fulfilled.
func IsIPReady(ip ipamv1alpha1.IP) bool {
	if ip.Status.IP == "" {
		return false
	}
	if ip.Status.CIDR == "" {
		return false
	}
	return true
}

// CheckIPCoherentWithNetwork Check if IP resource is coherent with its Network.
func CheckIPCoherentWithNetwork(ip ipamv1alpha1.IP, network ipamv1alpha1.Network) (bool, error) {
	if network.Status.CIDR != ip.Status.CIDR {
		return false, fmt.Errorf("IP %q is not in the network %q", ip.Status.IP, network.Status.CIDR)
	}

	prefix, err := netip.ParsePrefix(network.Status.CIDR.String())
	if err != nil {
		return false, fmt.Errorf("failed to parse prefix %q: %w", network.Status.CIDR, err)
	}

	addr, err := netip.ParseAddr(ip.Status.IP.String())
	if err != nil {
		return false, fmt.Errorf("failed to parse address %q: %w", ip.Status.IP, err)
	}

	if !prefix.Contains(addr) {
		return false, nil
	}

	return true, nil
}
