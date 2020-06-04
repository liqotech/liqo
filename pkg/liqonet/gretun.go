package liqonet

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"
	"net"
	"syscall"
)

type gretunAttributes struct {
	name   string
	local  net.IP
	remote net.IP
	ttl    uint8
}

type gretunIface struct {
	link *netlink.Gretun
}

func createGretunIface(iface *gretunIface) error {
	//Add the gretun interface
	err := netlink.LinkAdd(iface.link)
	if err == syscall.EEXIST {
		//Get the existing interface
		existingIface, err := netlink.LinkByName(iface.link.Name)
		if err != nil {
			return fmt.Errorf("failed to retrieve the existing gretun interface info: %v", err)
		}
		//Remove the existing gre interface
		if err = netlink.LinkDel(existingIface); err != nil {
			return fmt.Errorf("failed to delete the existing gretun interface: %v", err)
		}
		//Try do add again the gretun interface
		if err = netlink.LinkAdd(iface.link); err != nil {
			return fmt.Errorf("failed to re-create the gretun interface: %v", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to create the gretun interface: %v", err)
	}
	return nil
}

func newGretunInterface(attributes *gretunAttributes) (*gretunIface, error) {
	//filling the gretun struct with the right attributes
	iface := &netlink.Gretun{
		LinkAttrs: netlink.LinkAttrs{
			Name: attributes.name,
		},
		Local:  attributes.local,
		Remote: attributes.remote,
		Ttl:    attributes.ttl,
	}
	//assigning to the gretun interface the parameters
	gretunIface := &gretunIface{link: iface}
	//create the gretun interface
	if err := createGretunIface(gretunIface); err != nil {
		return nil, err
	}
	return gretunIface, nil

}

func (iface *gretunIface) deleteGretunIface() error {
	if err := netlink.LinkDel(iface.link); err != nil {
		return fmt.Errorf("failed to delete the gretun interface: %v", err)
	}
	return nil
}

func (iface *gretunIface) configureIPAddress(ipAddress net.IP, mask net.IPMask) error {
	ipConfig := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ipAddress,
			Mask: mask,
		},
	}
	//add the ip address to the gretun interface
	err := netlink.AddrAdd(iface.link, ipConfig)
	if err == syscall.EEXIST {
		klog.V(6).Infof("gretun interface (%s) has already IP (%s)", iface.link.Name, ipAddress)
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to configure IP address (%s) on gretun interface (%s). %v", ipAddress, iface.link.Name, err)
	}
	return nil
}

func (iface *gretunIface) setUp() error {
	err := netlink.LinkSetUp(iface.link)
	if err != nil {
		return fmt.Errorf("unable to bring up the interface (#{iface.link.name}). #{err}")
	}
	return nil
}
