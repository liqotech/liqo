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
	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"

	"github.com/liqotech/liqo/pkg/liqonet/errors"
)

var (
	hostVeth            = "originVeth"
	existingHostVeth    = "host-foo"
	gatewayVeth         = "dstVeth"
	existingGatewayVeth = "gateway-foo"
)

var _ = Describe("Netns", func() {

	JustAfterEach(func() {
		Expect(cleanUpEnv()).NotTo(HaveOccurred())
	})

	Describe("creating new network namespace", func() {
		Context("when network namespace does not exist and we want to create it", func() {
			It("should return a new network namespace and nil", func() {
				netnamespace, err := CreateNetns(netnsName)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(netnamespace).ShouldNot(BeNil())
				// Get the newly created namespace.
				netnsNew, err := netns.GetFromName(netnsName)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(netnsNew).ShouldNot(BeNil())
			})
		})

		Context("when network namespace does exist and we want to create it", func() {
			JustBeforeEach(func() {
				setUpNetns(netnsName)
			})

			It("should remove the old one and create a new one, returning the new netns and nil", func() {
				netnamespace, err := CreateNetns(netnsName)
				defer netnamespace.Close()
				Expect(err).ShouldNot(HaveOccurred())
				Expect(netnamespace).ShouldNot(BeNil())
				// Check that our dummy interface is not present in the newly created namespace.
				err = netnamespace.Do(func(ns ns.NetNS) error {
					links, err := netlink.LinkList()
					Expect(err).ShouldNot(HaveOccurred())
					Expect(len(links)).Should(BeNumerically("==", 1))
					return nil
				})
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("deleting a network namespace", func() {
		Context("when network namespace does exist and we want to remove it", func() {
			JustBeforeEach(func() {
				setUpNetns(netnsName)
			})

			It("should remove the existing namespace and return nil", func() {
				err := DeleteNetns(netnsName)
				Expect(err).ShouldNot(HaveOccurred())
				// Try to get the netns
				_, err = netns.GetFromName(netnsName)
				Expect(err).Should(Equal(unix.ENOENT))
			})
		})

		Context("when network namespace does not exist and we want to remeove it", func() {
			It("should do nothing and return nil", func() {
				err := DeleteNetns(netnsName)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("adding veth pair and moving one end to another network namespace", func() {
		Context("when network namespaces does exist", func() {
			JustBeforeEach(func() {
				setUpNetns(netnsName)
			})

			It("should add veth pair and return nil", func() {
				_, _, err := CreateVethPair(hostVeth, gatewayVeth, originNetns, newNetns, 1500)
				Expect(err).ShouldNot(HaveOccurred())
				// Get originVeth
				or, err := netlink.LinkByName(hostVeth)
				Expect(err).ShouldNot(HaveOccurred())
				// Get dstVeth
				err = newNetns.Do(func(netNS ns.NetNS) error {
					_, err = netlink.LinkByName(gatewayVeth)
					if err != nil {
						return err
					}
					return nil
				})
				Expect(err).ShouldNot(HaveOccurred())
				Expect(netlink.LinkDel(or)).ShouldNot(HaveOccurred())

			})
		})

		Context("when links with same name already exists", func() {
			JustBeforeEach(func() {
				setUpNetns(netnsName)
			})

			It("link exists in gateway netns, should return error", func() {
				_, _, err := CreateVethPair(hostVeth, existingGatewayVeth, originNetns, newNetns, 1500)
				Expect(err).Should(HaveOccurred())
			})

			It("link exists in host netns, should remove it and create again", func() {
				_, _, err := CreateVethPair(existingHostVeth, gatewayVeth, originNetns, newNetns, 1500)
				Expect(err).Should(BeNil())
			})
		})

		Context("when dst network namespace does not exist", func() {
			It("should return error", func() {
				_, _, err := CreateVethPair(hostVeth, gatewayVeth, nil, nil, 1500)
				Expect(err).Should(Equal(&errors.WrongParameter{
					Reason:    errors.NotNil,
					Parameter: "hostNetns and gatewayNetns",
				}))
			})
		})
	})
})
