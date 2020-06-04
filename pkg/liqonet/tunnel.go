package liqonet

import (
	"errors"
	"github.com/liqoTech/liqo/api/tunnel-endpoint/v1"
	"github.com/liqoTech/liqo/internal/errdefs"
	"github.com/prometheus/common/log"
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

//Get the LocalTunnelPublicIP which is exported to the pod through an environment
//variable called LocalTunnelPublicIP. The pod is run with hostNetwork=true so it gets the same IP
//of the host where it is scheduled. The IP is the same used by the kubelet to register
//to the API server
func GetLocalTunnelPublicIP() (net.IP, error) {
	ipAddress, isSet := os.LookupEnv("LOCAL_TUNNEL_PUBLIC_IP")
	if isSet == false {
		return nil, errdefs.NotFound("the pod IP is not set")
	}
	if ipAddress == "" {
		return nil, errors.New("pod IP is not yet set")
	}
	return net.ParseIP(ipAddress), nil
}

func GetLocalTunnelPublicIPToString() (string, error) {
	ipAddress, isSet := os.LookupEnv("LOCAL_TUNNEL_PUBLIC_IP")
	if isSet == false {
		return "", errdefs.NotFound("the pod IP is not set")
	}
	if ipAddress == "" {
		return "", errors.New("pod IP is not yet set")
	}
	return ipAddress, nil
}

func GetLocalTunnelPrivateIP() (net.IP, error) {
	ipAddress, isSet := os.LookupEnv("LOCAL_TUNNEL_PRIVATE_IP")
	if isSet == false {
		return nil, errdefs.NotFound("the pod IP is not set")
	}
	if ipAddress == "" {
		return nil, errors.New("pod IP is not yet set")
	}
	return net.ParseIP(ipAddress), nil
}

func GetLocalTunnelPrivateIPToString() (string, error) {
	ipAddress, isSet := os.LookupEnv("LOCAL_TUNNEL_PRIVATE_IP")
	if isSet == false {
		return "", errdefs.NotFound("the pod IP is not set")
	}
	if ipAddress == "" {
		return "", errors.New("pod IP is not yet set")
	}
	return ipAddress, nil
}

func InstallGreTunnel(endpoint *v1.TunnelEndpoint) (int, string, error) {
	//TODO configure the name according to the max length permitted by the kernel
	name := tunnelNamePrefix
	//get the local ip address and use it as local ip for the gre tunnel
	local, err := GetLocalTunnelPublicIP()
	if err != nil {
		return 0, "", err
	}
	remote := net.ParseIP(endpoint.Spec.TunnelPublicIP)
	ttl := tunnelTtl
	attr := gretunAttributes{
		name:   name,
		local:  local,
		remote: remote,
		ttl:    uint8(ttl),
	}
	gretunnel, err := newGretunInterface(&attr)
	if err != nil {
		return 0, "", err
	}
	ownPrivateIP, err := GetLocalTunnelPrivateIPToString()
	if err != nil {
		return 0, "", err
	}
	address, network, err := net.ParseCIDR(ownPrivateIP + "/32")
	if err != nil {
		return 0, "", err
	}
	err = gretunnel.configureIPAddress(address, network.Mask)
	if err != nil {
		return 0, "", err
	}
	if err = gretunnel.setUp(); err != nil {
		return 0, "", err
	}
	return gretunnel.link.Index, gretunnel.link.Name, nil
}

//this function is called to remove the gre tunnel external resource
//when the Custorm Resource is deleted. It has to be idempotent
func RemoveGreTunnel(endpoint *v1.TunnelEndpoint) error {
	//check if the interface index is set
	if endpoint.Status.TunnelIFaceIndex == 0 {
		log.Info("no tunnel installed. Do nothing")
		return nil
	} else {
		existingIface, err := GetIfaceByIndex(endpoint.Status.TunnelIFaceIndex)

		if err != nil {
			if err.Error() == "Link not found" {
				log.Error(err, "Interface not found")
				return nil
			}
			log.Error(err, "unable to retrieve tunnel interface")
			return err
		}
		//Remove the existing gre interface
		if err = netlink.LinkDel(existingIface); err != nil {
			log.Error(err, "unable to delete the tunnel after the tunnelEndpoint CR has been removed")
			return err
		}
	}
	return nil
}

func DeleteIFaceByIndex(ifaceIndex int) error {
	var err error = nil
	existingIface, err := netlink.LinkByIndex(ifaceIndex)
	if err != nil {
		log.Error(err, "unable to retrieve tunnel interface")
		return err
	}
	//Remove the existing gre interface
	if err = netlink.LinkDel(existingIface); err != nil {
		log.Error(err, "unable to delete the tunnel after the tunnelEndpoint CR has been removed")
		return err
	}
	return err
}

func GetIfaceByIndex(iFaceIndex int) (netlink.Link, error) {
	existingIface, err := netlink.LinkByIndex(iFaceIndex)

	if err != nil {
		log.Error(err, "unable to retrieve tunnel interface")
		return existingIface, err
	}
	return existingIface, err
}
