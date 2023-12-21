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

package route

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

// EnsureRoutesPresence ensures the presence of the given routes.
func EnsureRoutesPresence(routes []networkingv1alpha1.Route, tableID uint32) error {
	for i := range routes {
		route, err := forgeNetlinkRoute(&routes[i], tableID)
		if err != nil {
			return err
		}
		existingroute, exists, err := ExistsRoute(&routes[i], tableID)
		if err != nil {
			return err
		}
		if exists {
			if !IsEqualRoute(route, existingroute) {
				if err := netlink.RouteReplace(route); err != nil {
					return err
				}
			}
		} else {
			if err := netlink.RouteAdd(route); err != nil {
				return err
			}
		}
	}
	return nil
}

// EnsureRoutesAbsence ensures the absence of the given routes.
func EnsureRoutesAbsence(routes []networkingv1alpha1.Route, tableID uint32) error {
	for i := range routes {
		route, err := forgeNetlinkRoute(&routes[i], tableID)
		if err != nil {
			return err
		}
		_, exists, err := ExistsRoute(&routes[i], tableID)
		if err != nil {
			return err
		}
		if exists {
			if err := netlink.RouteDel(route); err != nil {
				return err
			}
		}
	}
	return nil
}

// ExistsRoute checks if the given route is already present in the route list.
func ExistsRoute(route *networkingv1alpha1.Route, tableID uint32) (*netlink.Route, bool, error) {
	_, dst, err := net.ParseCIDR(route.Dst.String())
	if err != nil {
		return nil, false, err
	}

	existingRoutes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{
		Dst:   dst,
		Table: int(tableID),
	}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, false, err
	}

	if len(existingRoutes) > 1 {
		return nil, false, fmt.Errorf("%v routes found with same destination", len(existingRoutes))
	}

	if len(existingRoutes) == 1 {
		return &existingRoutes[0], true, nil
	}

	return nil, false, nil
}

// IsEqualRoute checks if the two routes are equal.
func IsEqualRoute(route1, route2 *netlink.Route) bool {
	if route1.Dst != nil && route2.Dst != nil && route1.Dst.String() != route2.Dst.String() {
		return false
	}
	if route1.Src != nil && route2.Src != nil && route1.Src.String() != route2.Src.String() {
		return false
	}
	if route1.Gw != nil && route2.Gw != nil && route1.Gw.String() != route2.Gw.String() {
		return false
	}
	if route1.LinkIndex != route2.LinkIndex {
		return false
	}
	if route1.Flags != route2.Flags {
		return false
	}
	return true
}

// CleanRoutes cleans the routes that are not contained in the given route list.
func CleanRoutes(routes []networkingv1alpha1.Route, tableID uint32) error {
	existingrules, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{Table: int(tableID)}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}
	for i := range existingrules {
		if !IsContainedRoute(&existingrules[i], routes) {
			if err := netlink.RouteDel(&existingrules[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// IsContainedRoute checks if the given route is contained in the route list.
func IsContainedRoute(route *netlink.Route, routes []networkingv1alpha1.Route) bool {
	for i := range routes {
		r, err := forgeNetlinkRoute(&routes[i], uint32(route.Table))
		if err != nil {
			return false
		}
		if IsEqualRoute(route, r) {
			return true
		}
	}
	return false
}

func forgeNetlinkRoute(route *networkingv1alpha1.Route, tableID uint32) (*netlink.Route, error) {
	var flags int
	var err error
	var dst *net.IPNet
	var src, gw net.IP
	var linkIndex int

	if route.Dst != nil {
		_, dst, err = net.ParseCIDR(route.Dst.String())
		if err != nil {
			return nil, err
		}
	}

	if route.Src != nil {
		src = net.ParseIP(route.Src.String())
	}

	if route.Gw != nil {
		gw = net.ParseIP(route.Gw.String())
	}

	if route.Dev != nil {
		link, err := netlink.LinkByName(*route.Dev)
		if err != nil {
			return nil, err
		}
		linkIndex = link.Attrs().Index
	}

	if route.Onlink != nil && *route.Onlink {
		flags |= int(netlink.FLAG_ONLINK)
	}

	return &netlink.Route{
		Dst:       dst,
		Gw:        gw,
		Src:       src,
		LinkIndex: linkIndex,
		Table:     int(tableID),
		Flags:     flags,
	}, nil
}
