package wireguard

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

const (
	wgLinkType = "wireguard"
)

type Netlinker interface {
	getLinkIndex() int
	createLink(linkName string) error
	getLinkName() string
	addIP(ipAddr string) error
	setMTU(mtu int) error
}

type netlinkDevice struct {
	link netlink.Link
}

func NewNetLinker() Netlinker {
	return &netlinkDevice{link: nil}
}

func (nld *netlinkDevice) createLink(linkName string) error {
	var err error
	if link, err := netlink.LinkByName(linkName); err == nil {
		// delete existing device
		if err := netlink.LinkDel(link); err != nil {
			return fmt.Errorf("failed to delete existing wireguard device '%s': %v", linkName, err)
		}
	}
	// create the wg device (ip link add dev $DefaultlinkName type wireguard)
	la := netlink.NewLinkAttrs()
	la.Name = linkName
	link := &netlink.GenericLink{
		LinkAttrs: la,
		LinkType:  wgLinkType,
	}
	if err = netlink.LinkAdd(link); err != nil && err != unix.EOPNOTSUPP {
		return fmt.Errorf("failed to add wireguard device '%s': %v", linkName, err)
	}
	if err == unix.EOPNOTSUPP {
		klog.Warningf("wireguard kernel module not present, falling back to the userspace implementation")
		cmd := exec.Command("/usr/bin/boringtun", linkName, "--disable-drop-privileges", "true")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err != nil {
			outStr, errStr := stdout.String(), stderr.String()
			fmt.Printf("out:\n%s\nerr:\n%s\n", outStr, errStr)
			return fmt.Errorf("failed to add wireguard devices '%s': %v", linkName, err)
		}
		if nld.link, err = netlink.LinkByName(linkName); err != nil {
			return fmt.Errorf("failed to get wireguard device '%s': %v", linkName, err)
		}
	}
	nld.link = link
	// ip link set $w.getName up
	if err := netlink.LinkSetUp(nld.link); err != nil {
		return fmt.Errorf("failed to bring up wireguard device '%s': %v", linkName, err)
	}
	return nil
}

//adds the ip address to the interface
//ip address in cidr notation: x.x.x.x/x
func (nld *netlinkDevice) addIP(ipAddr string) error {
	ipNet, err := netlink.ParseIPNet(ipAddr)
	if err != nil {
		return err
	}
	err = netlink.AddrAdd(nld.link, &netlink.Addr{IPNet: ipNet})
	if err != nil {
		return fmt.Errorf("failed to add ip address %s to interface %s: %v", ipAddr, nld.link.Attrs().Name, err)
	}
	return nil
}

func (nld *netlinkDevice) setMTU(mtu int) error {
	if err := netlink.LinkSetMTU(nld.link, mtu); err != nil {
		return fmt.Errorf("failed to set mtu on interface %s: %v", nld.link.Attrs().Name, err)
	}
	return nil
}

// get link index of the wireguard device
func (nld *netlinkDevice) getLinkIndex() int {
	return nld.link.Attrs().Index
}

func (nld *netlinkDevice) getLinkName() string {
	return nld.link.Attrs().Name
}
