package overlay

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

// VxlanDeviceAttrs configuration for a new vxlan device.
type VxlanDeviceAttrs struct {
	Vni      int
	Name     string
	VtepPort int
	VtepAddr net.IP
	Mtu      int
}

// VxlanDevice struct that holds a vxlan link.
type VxlanDevice struct {
	Link *netlink.Vxlan
}

// Neighbor struct that holds information for an fdb entry.
type Neighbor struct {
	MAC net.HardwareAddr
	IP  net.IP
}

// NewVxlanDevice takes as argument a struct of type VxlanDeviceAttrs and returns a VxlanDevice or error.
func NewVxlanDevice(devAttrs *VxlanDeviceAttrs) (*VxlanDevice, error) {
	link := &netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:  devAttrs.Name,
			MTU:   devAttrs.Mtu,
			Flags: net.FlagUp,
		},
		VxlanId:  devAttrs.Vni,
		SrcAddr:  devAttrs.VtepAddr,
		Port:     devAttrs.VtepPort,
		Learning: true,
	}

	link, err := createVxLanLink(link)
	if err != nil {
		return nil, err
	}
	return &VxlanDevice{
		Link: link,
	}, nil
}

func createVxLanLink(link *netlink.Vxlan) (*netlink.Vxlan, error) {
	err := netlink.LinkAdd(link)
	if errors.Is(err, syscall.EEXIST) {
		existing, err := netlink.LinkByName(link.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve Link with name %s: %w", link.Name, err)
		}
		if isVxlanConfigTheSame(link, existing) {
			klog.V(4).Infof("vxlan device with the same configuration already exists")
			link = existing.(*netlink.Vxlan)
			return link, nil
		}
		// If we come here it means that the config of the existing vxlan device does not match with the new one.
		// We delete it and then recreate.
		if err = netlink.LinkDel(existing); err != nil {
			return nil, fmt.Errorf("failed to delete the existing vxlan device with name %s: %w", existing.Attrs().Name, err)
		}
		if err = netlink.LinkAdd(link); err != nil {
			return nil, fmt.Errorf("failed to re-create the vxlan interface: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to create the vxlan interface: %w", err)
	}
	ifindex := link.Index
	vxlan, err := netlink.LinkByIndex(ifindex)
	if err != nil {
		return nil, fmt.Errorf("can't locate created vxlan device with index %d, %w", ifindex, err)
	}
	return vxlan.(*netlink.Vxlan), nil
}

// If a vxlan exists with the same name then we use this function to check if the configuration is the same.
func isVxlanConfigTheSame(newLink, current netlink.Link) bool {
	if newLink.Type() != current.Type() {
		klog.V(4).Infof("different types for the interfaces: newLink -> %v, current -> %v", newLink.Type(), current.Type())
		return false
	}
	newNetlinkVxlan := newLink.(*netlink.Vxlan)
	currentNetlinkVxlan := current.(*netlink.Vxlan)

	if newNetlinkVxlan.VxlanId != currentNetlinkVxlan.VxlanId {
		klog.V(4).Infof("different vxlan ID for the interfaces: newLink -> %d, current -> %d", newNetlinkVxlan.VxlanId, currentNetlinkVxlan.VxlanId)
		return false
	}
	if !reflect.DeepEqual(newNetlinkVxlan.SrcAddr, currentNetlinkVxlan.SrcAddr) {
		klog.V(4).Infof("different Source Addresses for the interfaces: newLink -> %v, current -> %v", newNetlinkVxlan.SrcAddr, currentNetlinkVxlan.SrcAddr)
		return false
	}
	if newNetlinkVxlan.Port != currentNetlinkVxlan.Port {
		klog.V(4).Infof("different Vxlan Port for the interfaces: newLink -> %d, current -> %d", newNetlinkVxlan.Port, currentNetlinkVxlan.Port)
		return false
	}
	klog.V(4).Infof("the existing interface is already configured")
	return true
}

// AddFDB adds a fdb entry for the given neighbor into the current vxlan device.
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
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

// DelFDB deletes a fdb entry for the given neighbor from the current vxlan device.
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
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}
