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

package netns

import (
	"errors"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
)

// AddNeigh adds a permanent neighbor entry for the given neighbor into the given device.
// It returns an error if something goes wrong, and bool value set to true if it
// added the entry, otherwise is set to false.
func AddNeigh(addr net.IP, lladdr net.HardwareAddr, dev *net.Interface) (bool, error) {
	klog.V(5).Infof("calling ip neigh add %s lladdr %s dev %s state permanent", addr, lladdr.String(), dev.Name)
	// First we list all the neighbors
	neighbors, err := netlink.NeighList(dev.Index, syscall.AF_INET)
	if err != nil {
		return false, err
	}
	// Check if the entry exists.
	for i := range neighbors {
		if neighbors[i].IP.Equal(addr) && neighbors[i].HardwareAddr.String() == lladdr.String() {
			return false, nil
		}
	}
	err = netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    dev.Index,
		State:        netlink.NUD_PERMANENT,
		Family:       syscall.AF_INET,
		IP:           addr,
		HardwareAddr: lladdr,
	})
	if err != nil {
		return false, err
	}
	klog.V(5).Infof("neigh entry with mac {%s} and dst {%s} on device {%s} has been added",
		lladdr.String(), addr, dev.Name)
	return true, nil
}

// DelNeigh deletes a fdb entry for the given neighbor from the given device.
// It return an error if something goes wrong, and bool value set to true if it
// deleted the entry, if the entry does not exist the bool value is set to false.
func DelNeigh(addr net.IP, lladdr net.HardwareAddr, dev *net.Interface) (bool, error) {
	klog.V(5).Infof("calling ip neigh del %s lladdr %s dev %s", addr.String(), lladdr.String(), dev.Name)
	err := netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    dev.Index,
		State:        netlink.NUD_PERMANENT,
		Family:       syscall.AF_INET,
		IP:           addr,
		HardwareAddr: lladdr,
	})
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	klog.V(5).Infof("neigh entry with mac {%s} and dst {%s} on device {%s} has been removed",
		lladdr.String(), addr.String(), dev.Name)
	return true, nil
}
