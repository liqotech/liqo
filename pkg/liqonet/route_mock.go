package liqonet

import (
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"net"
	"reflect"
)

type MockRouteManager struct {
	RouteList []netlink.Route
}

func (m *MockRouteManager) AddRoute(dst string, gw string, deviceName string, onLink bool) (netlink.Route, error) {
	var route netlink.Route
	//convert destination in *net.IPNet
	_, destinationNet, err := net.ParseCIDR(dst)
	if err != nil {
		return route, err
	}
	gateway := net.ParseIP(gw)
	//here we keep the iface index at a fixed value
	ifaceIndex := 12

	//check if already exist a route for the destination network on our device
	//we don't care about other routes in devices not managed by net. The user should check the
	//possible ip conflicts
	if onLink {
		route = netlink.Route{LinkIndex: ifaceIndex, Dst: destinationNet, Gw: gateway, Flags: unix.RTNH_F_ONLINK}

		//here we add the route
		m.RouteList = append(m.RouteList, route)
	} else {
		route = netlink.Route{LinkIndex: ifaceIndex, Dst: destinationNet, Gw: gateway}
		//here we add the route
		m.RouteList = append(m.RouteList, route)
	}
	return route, nil
}

func (m *MockRouteManager) DelRoute(route netlink.Route) error {
	//try to remove all the routes for that ip
	for i, r := range m.RouteList {
		if reflect.DeepEqual(r, route) {
			m.RouteList = append(m.RouteList[:i], m.RouteList[i+1:]...)
		}
	}
	return nil
}
