package dronet_operator

import (
	"errors"
	v1 "github.com/netgroup-polito/dronev2/api/tunnel-endpoint/v1"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog"
	"net"
	"strings"
)

func AddRoute(dst *net.IPNet, gw net.IP, linkIndex int) (netlink.Route, error) {
	route := netlink.Route{ LinkIndex: linkIndex, Dst: dst, Gw: gw, Flags: unix.RTNH_F_ONLINK}
	if err := netlink.RouteAdd(&route); err != nil {
		return  route, err
	}
	return route, nil
}

//get the ip of the vxlan interface added by the flannel cni. this ip is
//the ip of the node where the tunnel operator runs
func GetGateway() ( int, net.IP, error) {
	var gw net.IP
	iface, err := net.InterfaceByName("flannel.1")
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

func DelRoute (route netlink.Route) error {
	//try to remove all the routes for that ip
		err := netlink.RouteDel(&route)
		if err != nil {
			klog.V(6).Info("unable to remove the route" + route.String())
			return err
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

func ValidateCRAndReturn(endpoint *v1.TunnelEndpoint) (podCIDR *net.IPNet, endNodeIP net.IP, err error){
	endpointIP := endpoint.Status.EndpointNodeIP
	if endpointIP == ""{
		err = errors.New("the endpoint ip is not set yet, unable to instantiate the route")
		return
	}
	endNodeIP = net.ParseIP(endpointIP)

	var remPodCIDR string
	if(endpoint.Spec.NATEnabled){
		remPodCIDR = endpoint.Spec.RemappedPodCIDR
	}else{
		remPodCIDR = endpoint.Spec.PodCIDR
	}
	_, podCIDR, err = net.ParseCIDR(remPodCIDR)
	if err != nil {
		return
	}
	err = nil
	return
}