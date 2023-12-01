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
	"github.com/vishvananda/netlink"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

// EnsureRoutesPresence ensures the presence of the given routes.
func EnsureRoutesPresence(routes []networkingv1alpha1.Route, tableID uint32) error {
	routesList, err := GetRoutesByTableID(tableID)
	if err != nil {
		return err
	}
	for i := range routes {
		exists := ExistsRoute(&routes[i], routesList)
		if err != nil {
			return err
		}
		_ = exists
		// TODO: add routes
	}
	return nil
}

// GetRoutesByTableID returns all the routes associated with the given table ID.
func GetRoutesByTableID(tableID uint32) ([]netlink.Route, error) {
	routelist, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, err
	}
	var routes []netlink.Route
	for i := range routelist {
		if routelist[i].Table == int(tableID) {
			routes = append(routes, routelist[i])
		}
	}
	return routes, nil
}

// ExistsRoute checks if the given route is already present in the route list.
func ExistsRoute(route *networkingv1alpha1.Route, routes []netlink.Route) bool {
	for i := range routes {
		if routes[i].Dst.String() == route.Dst.String() {
			return true
		}
	}
	return false
}

// IsEqualRoute checks if the given route is equal to the given netlink route.
func IsEqualRoute(route *networkingv1alpha1.Route, netlinkRoute *netlink.Route) (bool, error) {
	if route.Dst != nil && route.Dst.String() != netlinkRoute.Dst.String() {
		return false, nil
	}
	if route.Src != nil && route.Src.String() != netlinkRoute.Src.String() {
		return false, nil
	}
	if route.Gw != nil && route.Gw.String() != netlinkRoute.Gw.String() {
		return false, nil
	}
	if route.Dev != nil {
		link, err := netlink.LinkByName(*route.Dev)
		if err != nil {
			return false, err
		}
		if link.Attrs().Index != netlinkRoute.LinkIndex {
			return false, nil
		}
	}
	return true, nil
}
