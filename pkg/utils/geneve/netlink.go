// Copyright 2019-2024 The Liqo Authors
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

package geneve

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// EnsureGeneveInterfacePresence ensures that a geneve interface exists for the given internal node.
func EnsureGeneveInterfacePresence(interfaceName, localIP, remoteIP string, id uint32) error {
	remoteIPNet := net.ParseIP(remoteIP)
	if remoteIPNet == nil {
		remoteIPsNet, err := net.LookupIP(remoteIP)
		if err != nil {
			return err
		}
		remoteIPNet = remoteIPsNet[0]
	}
	return CreateGeneveInterface(interfaceName,
		net.ParseIP(localIP),
		remoteIPNet,
		id,
	)
}

// EnsureGeneveInterfaceAbsence ensures that a geneve interface does not exist for the given internal node.
func EnsureGeneveInterfaceAbsence(interfaceName string) error {
	link := ExistGeneveInterface(interfaceName)
	if link == nil {
		return nil
	}
	return netlink.LinkDel(link)
}

// CreateGeneveInterface creates a geneve interface with the given name, remote IP and ID.
func CreateGeneveInterface(name string, local, remote net.IP, id uint32) error {
	link := ExistGeneveInterface(name)
	if link == nil {
		link = &netlink.Geneve{
			LinkAttrs: netlink.LinkAttrs{
				Name:   name,
				TxQLen: 1000,
			},
			ID:     id,
			Remote: remote,
		}
		if err := netlink.LinkAdd(link); err != nil {
			return fmt.Errorf("cannot create geneve link: %w", err)
		}
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("cannot set geneve link up: %w", err)
	}

	if ExistGeneveInterfaceAddr(link, local) == nil {
		if err := netlink.AddrAdd(link, &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   local,
				Mask: net.IPMask{0xff, 0xff, 0xff, 0xff}, // Maybe it is not necessary. Remember to check.
			},
		}); err != nil {
			return fmt.Errorf("cannot add address to geneve link: %w", err)
		}
	}

	return nil
}

// ExistGeneveInterface checks if a geneve interface with the given name exists.
// If it exists, it returns the link, otherwise it returns nil.
func ExistGeneveInterface(name string) netlink.Link {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil
	}
	return link
}

// ExistGeneveInterfaceAddr checks if a geneve interface with the given name has the given address.
// If it exists, it returns the address, otherwise it returns nil.
func ExistGeneveInterfaceAddr(link netlink.Link, addr net.IP) *netlink.Addr {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return nil
	}
	for i := range addrs {
		if addrs[i].IP.Equal(addr) {
			return &addrs[i]
		}
	}
	return nil
}
