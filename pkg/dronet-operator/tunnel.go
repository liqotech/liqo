package dronet_operator

import (
	"errors"
	"github.com/netgroup-polito/dronev2/api/tunnel-endpoint/v1"
	"github.com/netgroup-polito/dronev2/internal/errdefs"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"
	"net"
	"os"
)

const (
	tunnelNamePrefix = "gretun_"
	tunnelTtl        = 255
)

// Get preferred outbound ip of this machine
func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	klog.V(6).Infof("local ip address of host is (%s)", localAddr.IP.String())
	return localAddr.IP, nil
}

//Get the Pod IP which is exported by the apiserver to the pod through an environment
//variable called POD_IP. The pod is run with hostNetwork=true so it gets the same IP
//of the host where it is scheduled. The IP is the same used by the kubelet to register
//to the API server
func GetHostIP () (net.IP, error) {
	podIP, isSet := os.LookupEnv("POD_IP")
	if isSet == false {
		return nil, errdefs.NotFound("the pod IP is not set")
	}
	if podIP == "" {
		return nil, errors.New("pod IP is not yet set")
	}
	return net.ParseIP(podIP), nil
}

func GetHostIPToString () (string, error) {
	podIP, isSet := os.LookupEnv("POD_IP")
	if isSet == false {
		return "", errdefs.NotFound("the pod IP is not set")
	}
	if podIP == "" {
		return "", errors.New("pod IP is not yet set")
	}
	return podIP, nil
}

func InstallGreTunnel(endpoint *v1.TunnelEndpoint) (int, error ){
	//TODO configure the name according to the max length permitted by the kernel
	name := tunnelNamePrefix
	//get the local ip address and use it as local ip for the gre tunnel
	local, err := GetHostIP()
	if err != nil {
		return 0, err
	}
	remote := net.ParseIP(endpoint.Spec.GatewayPublicIP)
	ttl := tunnelTtl
	attr := gretunAttributes{
		name:   name,
		local:  local,
		remote: remote,
		ttl:    uint8(ttl),
	}
	gretunnel, err := newGretunInterface(&attr)
	if err != nil {
		return 0, err
	}
	var ownPrivateIP string
	if endpoint.Spec.TunnelPrivateIP == "192.168.100.1" {
		ownPrivateIP = "192.168.100.2"
	}else {
		ownPrivateIP = "192.168.100.1"
	}
	address, network, err := net.ParseCIDR(ownPrivateIP + "/32")
	if err != nil {
		return 0, err
	}
	err = gretunnel.configureIPAddress(address, network.Mask)
	if err != nil {
		return 0, err
	}
	if err = gretunnel.setUp(); err != nil {
		return 0, err
	}
	dst := &net.IPNet{
		IP:   net.ParseIP(endpoint.Spec.TunnelPrivateIP),
		Mask: net.CIDRMask(32, 32),
	}
	route := netlink.Route{LinkIndex: gretunnel.link.Attrs().Index, Dst: dst}
	if err := netlink.RouteAdd(&route); err != nil {
		return 0, err
	}
	dst, _, err = ValidateCRAndReturn(endpoint)
	if err != nil{
		return 0, err
	}
	route = netlink.Route{LinkIndex: gretunnel.link.Attrs().Index, Dst: dst}
	if err := netlink.RouteAdd(&route); err != nil {
		return 0, err
	}

	return gretunnel.link.Index, nil
}
