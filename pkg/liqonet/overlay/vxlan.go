// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package overlay

import (
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
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
	MTU      int
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
			MTU:   devAttrs.MTU,
			Flags: net.FlagUp,
		},
		VxlanId:  devAttrs.Vni,
		SrcAddr:  devAttrs.VtepAddr,
		Port:     devAttrs.VtepPort,
		Learning: false,
	}
	link, err := createVxLanLink(link)
	if err != nil {
		return nil, err
	}
	v := &VxlanDevice{
		Link: link,
	}
	// Enable reverse path filtering for vxlan device.
	if err := v.enableRPFilter(); err != nil {
		return nil, err
	}
	return v, nil
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
		klog.V(4).Infof("different types for the interfaces: newLink -> %s, current -> %s", newLink.Type(), current.Type())
		return false
	}
	newNetlinkVxlan := newLink.(*netlink.Vxlan)
	currentNetlinkVxlan := current.(*netlink.Vxlan)

	if newNetlinkVxlan.VxlanId != currentNetlinkVxlan.VxlanId {
		klog.V(4).Infof("different vxlan ID for the interfaces: newLink -> %d, current -> %d", newNetlinkVxlan.VxlanId, currentNetlinkVxlan.VxlanId)
		return false
	}
	if !reflect.DeepEqual(newNetlinkVxlan.SrcAddr, currentNetlinkVxlan.SrcAddr) {
		klog.V(4).Infof("different Source Addresses for the interfaces: newLink -> %s, current -> %s", newNetlinkVxlan.SrcAddr, currentNetlinkVxlan.SrcAddr)
		return false
	}
	if newNetlinkVxlan.Port != currentNetlinkVxlan.Port {
		klog.V(4).Infof("different Vxlan Port for the interfaces: newLink -> %d, current -> %d", newNetlinkVxlan.Port, currentNetlinkVxlan.Port)
		return false
	}
	klog.V(4).Infof("the existing interface is already configured")
	return true
}

// ConfigureIPAddress configures the IP address of the vxlan interface.
// The IP address has to be in CIDR notation.
func (vxlan *VxlanDevice) ConfigureIPAddress(ipAddr string) error {
	// Parse the address.
	ipNet, err := netlink.ParseIPNet(ipAddr)
	if err != nil {
		return err
	}
	err = netlink.AddrAdd(vxlan.Link, &netlink.Addr{IPNet: ipNet})
	if errors.Is(err, unix.EEXIST) {
		klog.V(4).Infof("ip address %s is already configured on vxlan device %s", ipNet.String(), vxlan.Link.Name)
		return nil
	} else if err != nil {
		return err
	}
	klog.V(4).Infof("IP address %s configured on vxlan device %s", ipNet.String(), vxlan.Link.Name)
	return nil
}

// AddFDB adds a fdb entry for the given neighbor into the current vxlan device.
// It returns an error if something goes wrong, and bool value set to true if it
// added the entry, otherwise is set to false.
func (vxlan *VxlanDevice) AddFDB(n Neighbor) (bool, error) {
	klog.V(4).Infof("calling AppendFDB: %s, %s", n.IP, n.MAC)
	// First we list all the fdbs
	fdbs, err := netlink.NeighList(vxlan.Link.Index, syscall.AF_BRIDGE)
	if err != nil {
		return false, err
	}
	// Check if the entry exists.
	for i := range fdbs {
		if fdbs[i].IP.Equal(n.IP) && fdbs[i].HardwareAddr.String() == n.MAC.String() {
			return false, nil
		}
	}
	err = netlink.NeighAppend(&netlink.Neigh{
		LinkIndex:    vxlan.Link.Index,
		State:        netlink.NUD_PERMANENT | netlink.NUD_NOARP,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		Type:         netlink.NDA_DST,
		IP:           n.IP,
		HardwareAddr: n.MAC,
	})
	if err != nil {
		return false, err
	}
	klog.V(4).Infof("fdb entry with mac {%s} and dst {%s} on device {%s} has been added",
		n.MAC.String(), n.IP.String(), vxlan.Link.Name)
	return true, nil
}

// DelFDB deletes a fdb entry for the given neighbor from the current vxlan device.
// It return an error if something goes wrong, and bool value to sai if it
// deleted the entry, if the entry does not exist the bool value is set to false.
func (vxlan *VxlanDevice) DelFDB(n Neighbor) (bool, error) {
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
		return false, nil
	}
	if err != nil {
		return false, err
	}
	klog.V(4).Infof("fdb entry with mac {%s} and dst {%s} on device {%s} has been removed",
		n.MAC.String(), n.IP.String(), vxlan.Link.Name)
	return true, nil
}

// enableRPFilter sets the rp_filter in loose mode for the current overlay interface.
func (vxlan *VxlanDevice) enableRPFilter() error {
	ifaceName := vxlan.Link.Name
	klog.V(4).Infof("setting reverse path filtering for interface {%s} to loose mode", ifaceName)
	rpFilterFilePath := strings.Join([]string{"/proc/sys/net/ipv4/conf/", ifaceName, "/rp_filter"}, "")
	// Enable loose mode reverse path filtering on the overlay interface.
	err := os.WriteFile(rpFilterFilePath, []byte("2"), 0600)
	if err != nil {
		klog.Errorf("an error occurred while writing to file %s: %v", rpFilterFilePath, err)
		return err
	}
	return nil
}
