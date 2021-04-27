package overlay

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
	"net"
	"syscall"
)

type VxlanDeviceAttrs struct {
	Vni      uint32
	Name     string
	VtepPort int
	VtepAddr net.IP
	Mtu      int
}

type VxlanDevice struct {
	Link *netlink.Vxlan
}

type Neighbor struct {
	MAC net.HardwareAddr
	IP  net.IP
}

func NewVXLANDevice(devAttrs *VxlanDeviceAttrs) (*VxlanDevice, error) {
	link := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:  devAttrs.Name,
			MTU:   devAttrs.Mtu,
			Flags: net.FlagUp,
		},
		VxlanId:  int(devAttrs.Vni),
		SrcAddr:  devAttrs.VtepAddr,
		Port:     devAttrs.VtepPort,
		Learning: true,
	}

	link, err := CreateVxLanLink(link)
	if err != nil {
		return nil, err
	}
	return &VxlanDevice{
		Link: link,
	}, nil
}

func CreateVxLanLink(link *netlink.Vxlan) (*netlink.Vxlan, error) {
	err := netlink.LinkAdd(link)
	if err == syscall.EEXIST {
		existing, err := netlink.LinkByName(link.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve Link with name %s: %v", link.Name, err)
		}

		if IsVxlanConfigTheSame(link, existing) {
			klog.V(4).Infof("vxlan device with the same configuration already exists")
			link = existing.(*netlink.Vxlan)
			return link, nil
		}

		// if we come here it means that the config of the existing vxlan device does not match with the new one
		// we delete it and then recreate
		if err = netlink.LinkDel(existing); err != nil {
			return nil, fmt.Errorf("failed to delete the existing vxlan device with name %s: %v", existing.Attrs().Name, err)
		}

		if err = netlink.LinkAdd(link); err != nil {
			return nil, fmt.Errorf("failed to re-create the the vxlan interface: %v", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to create the the vxlan interface: %v", err)
	}

	ifindex := link.Index
	vxlan, err := netlink.LinkByIndex(ifindex)
	if err != nil {
		return nil, fmt.Errorf("can't locate created vxlan device with index %v, %v", ifindex, err)
	}

	return vxlan.(*netlink.Vxlan), nil
}

//if a vxlan exists with the same name then we
//use this function to check if the configuration is the same
func IsVxlanConfigTheSame(new, current netlink.Link) bool {

	if new.Type() != current.Type() {
		klog.V(4).Infof("different types for the interfaces: new -> %v, current -> %v", new.Type(), current.Type())
		return false
	}

	newNetlinkVxlan := new.(*netlink.Vxlan)
	currentNetlinkVxlan := current.(*netlink.Vxlan)

	if newNetlinkVxlan.VxlanId != currentNetlinkVxlan.VxlanId {
		klog.V(4).Infof("different VxlanID for the interfaces: new -> %d, current -> %d", newNetlinkVxlan.VxlanId, currentNetlinkVxlan.VxlanId)
		return false
	}

	if len(newNetlinkVxlan.SrcAddr) > 0 && len(currentNetlinkVxlan.SrcAddr) > 0 && !newNetlinkVxlan.SrcAddr.Equal(currentNetlinkVxlan.SrcAddr) {
		klog.V(4).Infof("different Source Addresses for the interfaces: new -> %v, current -> %v", newNetlinkVxlan.SrcAddr, currentNetlinkVxlan.SrcAddr)
		return false
	}

	if newNetlinkVxlan.Port > 0 && currentNetlinkVxlan.Port > 0 && newNetlinkVxlan.Port != currentNetlinkVxlan.Port {
		klog.V(4).Infof("different Vxlan Port for the interfaces: new -> %d, current -> %d", newNetlinkVxlan.Port, currentNetlinkVxlan.Port)
		return false
	}
	klog.V(4).Infof("the existing interface is already configured")
	return true
}

func (vxlan *VxlanDevice) ConfigureIPAddress(ip net.IP, ipNet *net.IPNet) error {
	address := &netlink.Addr{IPNet: &net.IPNet{
		IP:   ip,
		Mask: ipNet.Mask,
	}}

	err := netlink.AddrAdd(vxlan.Link, address)
	if err == syscall.EEXIST {
		klog.V(4).Infof("ip address %v is already configured on vxlan %v", address, vxlan.Link.Name)
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to configure address %s on vxlan interface %s. %v", address.IP, vxlan.Link.Name, err)
	}
	return nil
}

//delete the Link associated with a vxlan device
func (device *VxlanDevice) DeleteVxLanIface() error {
	err := netlink.LinkDel(device.Link)
	if err != nil {
		return fmt.Errorf("failed to delete the the vxlan interface with name: %s and ifaceIndex: %v: %v", device.Link.Name, device.Link.Index, err)
	}
	return nil
}

func (vxlan *VxlanDevice) AddFDB(n Neighbor) error {
	klog.V(4).Infof("calling AppendFDB: %v, %v", n.IP, n.MAC)
	err := netlink.NeighAdd(&netlink.Neigh{
		LinkIndex:    vxlan.Link.Index,
		State:        netlink.NUD_PERMANENT | netlink.NUD_NOARP,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		Type:         netlink.NDA_DST,
		IP:           n.IP,
		HardwareAddr: n.MAC,
	})
	if err == unix.EEXIST {
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

func (vxlan *VxlanDevice) DelFDB(n Neighbor) error {
	klog.V(4).Infof("calling DelFDB: %v, %v", n.IP, n.MAC)
	err := netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    vxlan.Link.Index,
		State:        netlink.NUD_PERMANENT | netlink.NUD_NOARP,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		Type:         netlink.NDA_DST,
		IP:           n.IP,
		HardwareAddr: n.MAC,
	})
	if err == unix.ESRCH {
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}
