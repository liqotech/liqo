package liqonet

import (
	"fmt"
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
		return route, fmt.Errorf("unable to convert destination \"%s\" from string to net.IPNet: %v", dst, err)
	}
	gateway := net.ParseIP(gw)
	//here we keep the iface index at a fixed value
	ifaceIndex := 12

	route = netlink.Route{LinkIndex: ifaceIndex, Dst: destinationNet, Gw: gateway}
	//check if already exist a route for the destination network on our device
	//we don't care about other routes in devices not managed by liqonet. The user should check the
	//possible ip conflicts
	routes := m.RouteList
	if len(routes) > 0 {
		//count how many routes exist for the the current destination
		//if more then one: something went wrong so we remove them all
		occurrences := 0
		for _, val := range routes {
			if val.Dst.String() == route.Dst.String() {
				occurrences++
			}
		}
		if occurrences > 1 {
			for _, val := range routes {
				err = m.DelRoute(val)
				if err != nil {
					return route, fmt.Errorf("unable to delete route %v:%v", val, err)
				}
			}
		} else if occurrences == 1 {
			index := 0
			for i, val := range routes {
				if val.Dst.String() == route.Dst.String() {
					index = i
				}
			}
			if IsRouteConfigTheSame(&routes[index], route) {
				return routes[index], nil
			}
		}
	}
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
