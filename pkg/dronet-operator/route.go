package dronet_operator

import (
	"fmt"
	v1 "github.com/netgroup-polito/dronev2/api/tunnel-endpoint/v1"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"net"
	"strings"
)

func AddRoute(dst string, gw string, deviceName string, onLink bool) (netlink.Route, error) {
	var route netlink.Route
	//convert destination in *net.IPNet
	destinationIP,destinationNet, err := net.ParseCIDR(dst)
	if err != nil{
		return route, fmt.Errorf("unable to convert destination \"%s\" from string to net.IPNet: %v", dst, err)
	}
	gateway := net.ParseIP(gw)
	iface, err := netlink.LinkByName(deviceName)
	if err != nil{
		return route, fmt.Errorf("unable to retrieve information of \"%s\": %v", deviceName, err)
	}
	route = netlink.Route{LinkIndex: iface.Attrs().Index, Dst: destinationNet, Gw: gateway}
	//check if already exist a route for the destination network on our device
	//we don't care about other routes in devices not managed by dronet. The user should check the
	//possible ip conflicts
	routes, err := netlink.RouteList(iface, netlink.FAMILY_V4)
	if err != nil{
		return route, fmt.Errorf("unable to get routes for \"%s\": %v", destinationIP.String(), err)
	}
	if len(routes)>0{
		//count how many routes exist for the the current destination
		//if more then one: something went wrong so we remove them all
		occurrences := 0
		for _, val := range routes{
			if val.Dst.String() == route.Dst.String(){
				occurrences ++
			}
		}
		if occurrences > 1{
			for _, val := range routes{
				err = DelRoute(val)
				if err != nil{
					return route, fmt.Errorf("unable to delete route %v:%v", val, err)
				}
			}
		}else if occurrences ==1 {
			index := 0
			for i, val := range routes{
				if val.Dst.String() == route.Dst.String(){
					index = i
				}
			}
			if IsRouteConfigTheSame(&routes[index], route){
				return routes[index], nil
			}
		}
	}
	if onLink{
		route = netlink.Route{LinkIndex: iface.Attrs().Index, Dst: destinationNet, Gw: gateway, Flags: unix.RTNH_F_ONLINK}

		if err := netlink.RouteAdd(&route); err != nil {
			return route, fmt.Errorf("unable to instantiate route for %s  network with gateway %s:%v", dst, gw, err)
		}
	}else{
		route = netlink.Route{LinkIndex: iface.Attrs().Index, Dst: destinationNet, Gw: gateway}
		if err := netlink.RouteAdd(&route); err != nil {
			return route, fmt.Errorf("unable to instantiate route for %s  network with gateway %s:%v", dst, gw, err)
		}
	}
	return route, nil
}

func IsRouteConfigTheSame(existing *netlink.Route, new netlink.Route) bool{
	if existing.LinkIndex == new.LinkIndex && existing.Gw.String() == new.Gw.String() && existing.Dst.String() == new.Dst.String(){
		return true
	}else{
		return false
	}
}


//get the ip of the vxlan interface added by the flannel cni. this ip is
//the ip of the node where the tunnel operator runs
func GetGateway() (int, net.IP, error) {
	var gw net.IP
	iface, err := net.InterfaceByName("wlp1s0")
	if err != nil {
		return 0, gw, nil
	}
	addresses, err := iface.Addrs()
	if err != nil {
		return 0, gw, nil
	}
	s := strings.Split(addresses[0].String(), "/")
	gw = net.ParseIP(s[0])
	return iface.Index, gw, nil
}

func DelRoute(route netlink.Route) error {

	//try to remove all the routes for that ip
	err := netlink.RouteDel(&route)
	if err != nil {
		if err == unix.ESRCH{
			//it means the route does not exist so we are done
			return nil
		}
		return fmt.Errorf("unable to delete route %v: %v", route, err)
	}
	return nil
}

func StringtoIPNet(ipNet string) (net.IP, error) {
	ip, _, err := net.ParseCIDR(ipNet)
	if err != nil {
		return nil, err
	}
	return ip, nil
}
//checks if all the values need to install routes have ben set in the CR status
func ValidateCRAndReturn(endpoint *v1.TunnelEndpoint) (bool) {
	isReady := true
	if endpoint.Status.NATEnabled{
		if endpoint.Status.RemappedPodCIDR == ""{
			isReady = false
		}
	}
	if endpoint.Status.RemoteTunnelPrivateIP == "" || endpoint.Status.RemoteTunnelPublicIP == ""{
		isReady = false
	}
	if endpoint.Status.LocalTunnelPrivateIP == "" || endpoint.Status.LocalTunnelPublicIP == "" {
		isReady = false
	}
	return isReady
}
