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
	"fmt"
	"net"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
)

var (
	defaultIfaceIP net.IP
)

func TestOverlay(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Overlay Suite")
}

var _ = BeforeSuite(func() {
	var err error
	defaultIfaceIP, err = getIFaceIP()
	Expect(err).ShouldNot(HaveOccurred())
	Expect(defaultIfaceIP).ShouldNot(BeNil())
	// Create dummy link
	err = netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "dummy-link"}})
	Expect(err).ShouldNot(HaveOccurred())
})

func getIFaceIP() (net.IP, error) {
	var ifaceIndex int
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	// Find the route whose destination contains our IP address.
	for i := range routes {
		if routes[i].Scope == netlink.SCOPE_UNIVERSE {
			ifaceIndex = routes[i].LinkIndex
		}
	}
	if ifaceIndex == 0 {
		return nil, fmt.Errorf("unable to get ip for default interface")
	}
	// Get link.
	link, err := netlink.LinkByIndex(ifaceIndex)
	if err != nil {
		return nil, err
	}
	ips, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	return ips[0].IP, nil
}
