// Copyright 2019-2025 The Liqo Authors
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
	"k8s.io/klog/v2"
)

// EnsureGeneveInterfacePresence ensures that a geneve interface exists for the given internal node.
func EnsureGeneveInterfacePresence(interfaceName, localIP, remoteIP string, id uint32, disableARP bool, mtu int, port uint16) error {
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
		disableARP,
		mtu,
		port,
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

// ForgeGeneveInterface creates a geneve interface with the given name, remote IP and ID.
func ForgeGeneveInterface(name string, remote net.IP, id uint32, mtu int, port uint16) *netlink.Geneve {
	return &netlink.Geneve{
		LinkAttrs: netlink.LinkAttrs{
			Name:   name,
			TxQLen: 1000,
			MTU:    mtu,
		},
		ID:     id,
		Remote: remote,
		Dport:  port,
	}
}

// CreateGeneveInterface creates a geneve interface with the given name, remote IP and ID.
func CreateGeneveInterface(name string, local, remote net.IP, id uint32, disableARP bool, mtu int, port uint16) error {
	var geneveLink *netlink.Geneve
	link := ExistGeneveInterface(name)

	if link == nil {
		geneveLink = ForgeGeneveInterface(name, remote, id, mtu, port)
		if err := netlink.LinkAdd(geneveLink); err != nil {
			return fmt.Errorf("cannot create geneve link: %w", err)
		}
	} else {
		geneveLink = link.(*netlink.Geneve)
		if !geneveLink.Remote.Equal(remote) || geneveLink.MTU != mtu || geneveLink.Dport != port {
			klog.Warningf("geneve link already exists with different remote IP (%s -> %s), modifyng it",
				geneveLink.Remote.String(), remote.String())
			if err := netlink.LinkDel(geneveLink); err != nil {
				return fmt.Errorf("cannot delete geneve link: %w", err)
			}
			geneveLink = ForgeGeneveInterface(name, remote, id, mtu, port)
			if err := netlink.LinkAdd(geneveLink); err != nil {
				return fmt.Errorf("cannot modify geneve link: %w", err)
			}
		}
	}

	if disableARP {
		if err := netlink.LinkSetARPOff(geneveLink); err != nil {
			return fmt.Errorf("cannot set geneve link arp off: %w", err)
		}
	}

	if err := netlink.LinkSetUp(geneveLink); err != nil {
		return fmt.Errorf("cannot set geneve link up: %w", err)
	}

	if ExistGeneveInterfaceAddr(geneveLink, local) == nil {
		if err := netlink.AddrAdd(geneveLink, &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   local,
				Mask: net.IPMask{0xff, 0xff, 0xff, 0xff},
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

// ListGeneveInterfaces returns all the geneve interfaces.
func ListGeneveInterfaces() ([]netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("cannot list geneve links: %w", err)
	}
	var geneveLinks []netlink.Link
	for i := range links {
		if links[i].Type() == "geneve" {
			geneveLinks = append(geneveLinks, links[i])
		}
	}
	return geneveLinks, nil
}
