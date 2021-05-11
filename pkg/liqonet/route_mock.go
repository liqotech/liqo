package liqonet

import (
	"net"
	"reflect"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// MockRouteManager fake implementation of the route manager used for testing purposes.
type MockRouteManager struct {
	RouteList []netlink.Route
}

// AddRoute adds a new route.
func (m *MockRouteManager) AddRoute(dst, gw, deviceName string, onLink bool) (netlink.Route, error) {
	var route netlink.Route
	// convert destination in *net.IPNet
	_, destinationNet, err := net.ParseCIDR(dst)
	if err != nil {
		return route, err
	}
	gateway := net.ParseIP(gw)
	// here we keep the iface index at a fixed value
	ifaceIndex := 12

	// check if already exist a route for the destination network on our device
	// we don't care about other routes in devices not managed by net. The user should check the
	// possible ip conflicts
	if onLink {
		route = netlink.Route{LinkIndex: ifaceIndex, Dst: destinationNet, Gw: gateway, Flags: unix.RTNH_F_ONLINK}

		// here we add the route
		m.RouteList = append(m.RouteList, route)
	} else {
		route = netlink.Route{LinkIndex: ifaceIndex, Dst: destinationNet, Gw: gateway}
		// here we add the route
		m.RouteList = append(m.RouteList, route)
	}
	return route, nil
}

// DelRoute removes a route.
func (m *MockRouteManager) DelRoute(route *netlink.Route) error {
	// try to remove all the routes for that ip
	for i := range m.RouteList {
		if reflect.DeepEqual(m.RouteList[i], route) {
			m.RouteList = append(m.RouteList[:i], m.RouteList[i+1:]...)
		}
	}
	return nil
}
