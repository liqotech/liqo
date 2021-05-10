package wireguard

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

type netlinkDeviceFake struct {
	link          netlink.Link
	ip            net.IPNet
	mtu           int
	errorOnCreate bool
	errorOnAddIP  bool
	errorOnSetMTU bool
}

func NewNetLinkerFake(errOnCreate, errOnAddIP, errOnSetMtu bool) Netlinker {
	return &netlinkDeviceFake{link: nil,
		errorOnCreate: errOnCreate,
		errorOnAddIP:  errOnAddIP,
		errorOnSetMTU: errOnSetMtu,
	}
}

func (nld *netlinkDeviceFake) createLink(linkName string) error {
	// create the wg device (ip link add dev $DefaultlinkName type wireguard)
	la := netlink.NewLinkAttrs()
	la.Name = linkName
	la.Index = 123
	nld.link = &netlink.GenericLink{
		LinkAttrs: la,
		LinkType:  wgLinkType,
	}
	if nld.errorOnCreate {
		return fmt.Errorf("error generated for testing purpose")
	}
	return nil
}

//adds the ip address to the interface
//ip address in cidr notation: x.x.x.x/x
func (nld *netlinkDeviceFake) addIP(ipAddr string) error {
	ip, err := netlink.ParseIPNet(ipAddr)
	nld.ip = *ip
	if err != nil {
		return err
	}
	if nld.errorOnAddIP {
		return fmt.Errorf("error generated for testing purposes")
	}
	return nil
}

func (nld *netlinkDeviceFake) setMTU(mtu int) error {
	nld.mtu = mtu
	if nld.errorOnSetMTU {
		return fmt.Errorf("error generated for testing purposes")
	}
	return nil
}

// get link index of the wireguard device
func (nld *netlinkDeviceFake) getLinkIndex() int {
	return nld.link.Attrs().Index
}

func (nld *netlinkDeviceFake) getLinkName() string {
	return nld.link.Attrs().Name
}
