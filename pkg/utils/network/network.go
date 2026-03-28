// Copyright 2019-2026 The Liqo Authors
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

package network

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// GetDefaultInterfaceName returns the name of the interface used for the default route.
func GetDefaultInterfaceName() (string, error) {
	// List all routes in the main table without filtering by destination.
	// We filter manually because netlink v1.3.0 changed how default routes are represented
	// (from Dst=nil to explicit 0.0.0.0/0 or ::/0), and the RT_FILTER_DST behavior changed.
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{}, 0)
	if err != nil {
		return "", err
	}

	// Filter for default routes (0.0.0.0/0 or ::/0)
	var defaultRoute *netlink.Route
	for i := range routes {
		if isDefaultRoute(&routes[i]) {
			defaultRoute = &routes[i]
			break
		}
	}

	if defaultRoute == nil {
		return "", fmt.Errorf("no default route found")
	}

	link, err := netlink.LinkByIndex(defaultRoute.LinkIndex)
	if err != nil {
		return "", err
	}
	if link == nil {
		return "", fmt.Errorf("no default interface found")
	}
	return link.Attrs().Name, nil
}

// isDefaultRoute returns true if the route is a default route (0.0.0.0/0 or ::/0).
func isDefaultRoute(route *netlink.Route) bool {
	if route.Dst == nil {
		// Backward compatibility: in older netlink versions, default routes had nil Dst
		return true
	}

	// In netlink v1.3.0+, default routes have explicit Dst values
	ones, bits := route.Dst.Mask.Size()
	if ones != 0 {
		// Not a /0 route
		return false
	}

	// Check if IP is all zeros (0.0.0.0 for IPv4 or :: for IPv6)
	return route.Dst.IP.IsUnspecified() && (bits == 32 || bits == 128)
}
