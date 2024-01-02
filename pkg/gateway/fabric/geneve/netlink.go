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

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

// EnsureGeneveInterfacePresence ensures that a geneve interface exists for the given internal node.
func EnsureGeneveInterfacePresence(internalnode *networkingv1alpha1.InternalNode, opts *Options) error {
	interfaceName := internalnode.Spec.Interface.Gateway.Name
	local := net.ParseIP(GeneveGatewayInterfaceIP)
	nodeIP := net.ParseIP(internalnode.Spec.NodeAddress)
	if nodeIP == nil {
		nodeIPs, err := net.LookupIP(internalnode.Spec.NodeAddress)
		if err != nil {
			return err
		}
		nodeIP = nodeIPs[0]
	}
	return CreateGeneveInterface(interfaceName, local, nodeIP, opts)
}

// CreateGeneveInterface creates a geneve interface with the given name, remote IP and ID.
func CreateGeneveInterface(name string, local, remote net.IP, opts *Options) error {
	link := ExistGeneveInterface(name)
	if link == nil {
		link = &netlink.Geneve{
			LinkAttrs: netlink.LinkAttrs{
				Name: name,
			},
			ID:     opts.GeneveID,
			Remote: remote,
		}
		if err := netlink.LinkAdd(link); err != nil {
			fmt.Printf("cannot create geneve link: %s", err)
		}
	}

	if ExistGeneveInterfaceAddr(link, local) == nil {
		if err := netlink.AddrAdd(link, &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   local,
				Mask: net.IPMask{0xff, 0xff, 0xff, 0xff}, // Maybe it is not necessary. Remember to check.
			},
		}); err != nil {
			fmt.Printf("cannot add address to geneve link: %s", err)
		}
	}

	if err := netlink.LinkSetUp(link); err != nil {
		fmt.Printf("cannot set geneve link up: %s", err)
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
