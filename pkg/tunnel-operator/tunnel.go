package tunnel_operator

import (
	"github.com/netgroup-polito/dronev2/api/tunnel-endpoint/v1"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"
	"net"
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

func InstallGreTunnel(endpoint *v1.TunnelEndpoint) error {
	//TODO configure the name according to the max length permitted by the kernel
	name := tunnelNamePrefix
	//get the local ip address and use it as local ip for the gre tunnel
	local, err := getOutboundIP()
	if err != nil {
		return err
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
		return err
	}
	address, network, err := net.ParseCIDR(endpoint.Spec.TunnelPrivateIP + "/32")
	if err != nil {
		return err
	}
	err = gretunnel.configureIPAddress(address, network.Mask)
	if err != nil {
		return err
	}
	if err = gretunnel.setUp(); err != nil {
		return err
	}
	dst := &net.IPNet{
		IP:   net.ParseIP("10.0.1.1"),
		Mask: net.CIDRMask(32, 32),
	}
	route := netlink.Route{LinkIndex: gretunnel.link.Attrs().Index, Dst: dst}
	if err := netlink.RouteAdd(&route); err != nil {
		return err
	}
	return nil
}
