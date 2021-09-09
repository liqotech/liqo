// Copyright 2019-2021 The Liqo Authors
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

package tunneloperator

import (
	"net"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("TunnelOperator", func() {
	Describe("setup gateway namespace", func() {
		Context("configuring the new gateway namespace", func() {
			JustAfterEach(func() {
				link, err := netlink.LinkByName(liqoconst.HostVethName)
				if err != nil {
					Expect(err).Should(MatchError("Link not found"))
				}
				if err != nil && err.Error() != "Link not found" {
					Expect(err).ShouldNot(HaveOccurred())
				}
				if link != nil {
					Expect(netlink.LinkDel(link)).ShouldNot(HaveOccurred())
				}
			})
			It("should return nil", func() {
				err := tc.setUpGWNetns(liqoconst.GatewayNetnsName, liqoconst.HostVethName, liqoconst.GatewayVethName,
					"169.254.1.134/32", 1420)
				Expect(err).ShouldNot(HaveOccurred())
				// Check that we have the veth interface in host namespace
				err = tc.hostNetns.Do(func(ns ns.NetNS) error {
					defer GinkgoRecover()
					link, err := netlink.LinkByName(liqoconst.HostVethName)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(link.Attrs().MTU).Should(BeNumerically("==", 1420))
					return nil
				})
				// Check that we have the veth interface in gateway namespace
				err = tc.gatewayNetns.Do(func(ns ns.NetNS) error {
					defer GinkgoRecover()
					link, err := netlink.LinkByName(liqoconst.GatewayVethName)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(link.Attrs().MTU).Should(BeNumerically("==", 1420))
					addresses, err := netlink.AddrList(link, netlink.FAMILY_V4)
					Expect(addresses[0].IPNet.String()).Should(Equal("169.254.1.134/32"))
					Expect(err).ShouldNot(HaveOccurred())

					return nil
				})
			})

			It("incorrect name for veth interface, should return error", func() {
				err := tc.setUpGWNetns(liqoconst.GatewayNetnsName, "", liqoconst.GatewayVethName, "169.254.1.134/24", 1420)
				Expect(err).Should(HaveOccurred())
				Expect(err).Should(MatchError("failed to make veth pair: LinkAttrs.Name cannot be empty"))
			})

			It("incorrect ip address for veth interface, should return error", func() {
				err := tc.setUpGWNetns(liqoconst.GatewayNetnsName, liqoconst.HostVethName, liqoconst.GatewayVethName, "169.254.1.1.34/24", 1420)
				Expect(err).Should(HaveOccurred())
				Expect(err).Should(MatchError(&net.ParseError{Text: "169.254.1.1.34/24", Type: "CIDR address"}))
			})
		})
	})
})
