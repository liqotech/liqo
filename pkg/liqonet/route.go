package liqonet

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"net"
)

type NetLink interface {
	AddRoute(dst string, gw string, deviceName string, onLink bool) (netlink.Route, error)
	DelRoute(route netlink.Route) error
}

type RouteManager struct {
}

func (rm *RouteManager) AddRoute(dst string, gw string, deviceName string, onLink bool) (netlink.Route, error) {
	var route netlink.Route
	//convert destination in *net.IPNet
	_, destinationNet, err := net.ParseCIDR(dst)
	if err != nil {
		return route, fmt.Errorf("unable to convert destination \"%s\" from string to net.IPNet: %v", dst, err)
	}
	gateway := net.ParseIP(gw)
	iface, err := netlink.LinkByName(deviceName)
	if err != nil {
		return route, fmt.Errorf("unable to retrieve information of \"%s\": %v", deviceName, err)
	}
	if onLink {
		route = netlink.Route{LinkIndex: iface.Attrs().Index, Dst: destinationNet, Gw: gateway, Flags: unix.RTNH_F_ONLINK}

		if err := netlink.RouteAdd(&route); err != nil {
			return route, fmt.Errorf("unable to instantiate route for %s  network with gateway %s:%v", dst, gw, err)
		}
	} else {
		route = netlink.Route{LinkIndex: iface.Attrs().Index, Dst: destinationNet, Gw: gateway}
		if err := netlink.RouteAdd(&route); err != nil {
			return route, fmt.Errorf("unable to instantiate route for %s  network with gateway %s:%v", dst, gw, err)
		}
	}
	return route, nil
}

func IsRouteConfigTheSame(existing *netlink.Route, new netlink.Route) bool {
	if existing.LinkIndex == new.LinkIndex && existing.Gw.String() == new.Gw.String() && existing.Dst.String() == new.Dst.String() {
		return true
	} else {
		return false
	}
}

func (rm *RouteManager) DelRoute(route netlink.Route) error {
	//try to remove all the routes for that ip
	err := netlink.RouteDel(&route)
	if err != nil {
		if err == unix.ESRCH {
			//it means the route does not exist so we are done
			return nil
		}
		return err
	}
	return nil
}
