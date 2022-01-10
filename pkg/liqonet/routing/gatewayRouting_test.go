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

package routing

import (
	"net"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoerrors "github.com/liqotech/liqo/pkg/liqonet/errors"
)

var (
	routingTableIDGRM = 19773
	routesGRM         = []routingInfo{
		{
			destinationNet: "10.120.0.0/16",
			routingTableID: routingTableIDGRM,
		},
		{
			destinationNet: "10.121.0.0/16",
			routingTableID: routingTableIDGRM,
		}}
	existingRoutesGRM []*netlink.Route
	tepGRM            netv1alpha1.TunnelEndpoint
	tunnelDevice      netlink.Link
)

var _ = Describe("GatewayRouting", func() {
	BeforeEach(func() {
		// Populate the index of the routes with the correct one.
		for i := range routesGRM {
			routesGRM[i].iFaceIndex = tunnelDevice.Attrs().Index
		}
	})

	Describe("creating new Gateway Route Manager", func() {

		Context("when parameters are not valid", func() {
			It("routingTableID parameter out of range: a negative number", func() {
				grm, err := NewGatewayRoutingManager(-244, tunnelDevice)
				Expect(grm).Should(BeNil())
				Expect(err).Should(Equal(&liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.GreaterOrEqual + strconv.Itoa(0)}))
			})

			It("routingTableID parameter out of range: superior to max value ", func() {
				grm, err := NewGatewayRoutingManager(unix.RT_TABLE_MAX+1, tunnelDevice)
				Expect(grm).Should(BeNil())
				Expect(err).Should(Equal(&liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.MinorOrEqual + strconv.Itoa(unix.RT_TABLE_MAX)}))
			})
			It("tunnelDevice is nil", func() {
				grm, err := NewGatewayRoutingManager(routingTableIDGRM, nil)
				Expect(grm).Should(BeNil())
				Expect(err).Should(Equal(&liqoerrors.WrongParameter{Parameter: "tunnelDevice", Reason: liqoerrors.NotNil}))
			})
		})

		Context("when parameters are correct", func() {
			It("right parameters", func() {
				grm, err := NewGatewayRoutingManager(routingTableIDGRM, tunnelDevice)
				Expect(grm).ShouldNot(BeNil())
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("configuring routes for a remote peering cluster", func() {
		JustBeforeEach(func() {
			tepGRM = netv1alpha1.TunnelEndpoint{
				Spec: netv1alpha1.TunnelEndpointSpec{
					LocalNATPodCIDR:       "10.150.0.0/16",
					RemoteNATPodCIDR:      "10.250.0.0/16",
					RemoteExternalCIDR:    "10.151.0.0/16",
					RemoteNATExternalCIDR: "10.251.0.0/16",
				},
				Status: netv1alpha1.TunnelEndpointStatus{
					VethIFaceIndex: 12345,
					GatewayIP:      ipAddress2NoSubnet,
				}}
		})
		Context("when tep holds malformed parameters", func() {
			It("route configuration fails while adding route for PodCIDR", func() {
				tepGRM.Spec.RemoteNATPodCIDR = "10.150.000/16"
				added, err := grm.EnsureRoutesPerCluster(&tepGRM)
				Expect(err).Should(Equal(&net.ParseError{
					Type: "CIDR address",
					Text: "10.150.000/16",
				}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})
			It("route configuration fails while adding route for ExternalCIDR", func() {
				tepGRM.Spec.RemoteNATExternalCIDR = "10.151.000/16"
				added, err := grm.EnsureRoutesPerCluster(&tepGRM)
				Expect(err).Should(Equal(&net.ParseError{
					Type: "CIDR address",
					Text: "10.151.000/16",
				}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when tep holds correct parameters", func() {
			JustBeforeEach(func() {
				existingRoutesGRM = setUpRoutes(routesGRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDGRM)
			})

			It("route configuration should be correctly inserted", func() {
				added, err := grm.EnsureRoutesPerCluster(&tep)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeTrue())
				// Get the inserted routes
				_, dstPodCIDRNet, err := net.ParseCIDR(tep.Spec.RemoteNATPodCIDR)
				Expect(err).ShouldNot(HaveOccurred())
				_, dstExternalCIDRNet, err := net.ParseCIDR(tep.Spec.RemoteNATPodCIDR)
				Expect(err).ShouldNot(HaveOccurred())
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: dstPodCIDRNet,
					Table: routingTableIDGRM}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(routes[0].Dst.String()).Should(Equal(tep.Spec.RemoteNATPodCIDR))
				Expect(routes[0].Gw).Should(BeNil())
				routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: dstExternalCIDRNet,
					Table: routingTableIDGRM}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(routes[0].Dst.String()).Should(Equal(tep.Spec.RemoteNATPodCIDR))
				Expect(routes[0].Gw).Should(BeNil())
			})

			It("route already exists, should return false and nil", func() {
				tepGRM.Spec.RemoteNATPodCIDR = existingRoutesGRM[0].Dst.String()
				tepGRM.Spec.RemoteNATExternalCIDR = existingRoutesGRM[1].Dst.String()
				tepGRM.Status.GatewayIP = existingRoutesGRM[0].Gw.String()
				added, err := grm.EnsureRoutesPerCluster(&tepGRM)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeFalse())
			})
		})
	})

	Describe("removing route configuration for a remote peering cluster", func() {
		Context("when tep holds malformed parameters", func() {
			It("fails to remove route configuration while removing the route for PodCIDR", func() {
				tepGRM.Spec.RemoteNATPodCIDR = ""
				added, err := grm.RemoveRoutesPerCluster(&tepGRM)
				Expect(err).Should(Equal(&net.ParseError{
					Type: "CIDR address",
					Text: "",
				}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})
			It("fails to remove route configuration while removing the route for ExternalCIDR", func() {
				tepGRM.Spec.RemoteNATExternalCIDR = ""
				added, err := grm.RemoveRoutesPerCluster(&tepGRM)
				Expect(err).Should(Equal(&net.ParseError{
					Type: "CIDR address",
					Text: "",
				}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when tep holds correct parameters", func() {
			JustBeforeEach(func() {
				existingRoutesGRM = setUpRoutes(routesGRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDGRM)
			})

			It("route configuration should be correctly removed", func() {
				tepGRM.Spec.RemoteNATPodCIDR = existingRoutesGRM[0].Dst.String()
				tepGRM.Spec.RemoteNATExternalCIDR = existingRoutesGRM[1].Dst.String()
				deleted, err := grm.RemoveRoutesPerCluster(&tepGRM)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(deleted).Should(BeTrue())
				// Try to get remove routes.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesGRM[0], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
				routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesGRM[1], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
			})

			It("route configuration should be correctly removed from veth pair", func() {
				tepGRM.Spec.RemoteNATPodCIDR = existingRoutesGRM[1].Dst.String()
				tepGRM.Spec.RemoteNATExternalCIDR = existingRoutesGRM[0].Dst.String()
				tepGRM.Status.GatewayIP = ipAddress1NoSubnet
				tepGRM.Status.VethIFaceIndex = overlayDevice.Link.Index
				added, err := grm.RemoveRoutesPerCluster(&tepGRM)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeTrue())
				// Try to get remove routes.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesGRM[1], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
				routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesGRM[0], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
			})

			It("route does not exist, should return false and nil", func() {
				added, err := grm.RemoveRoutesPerCluster(&tep)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeFalse())
			})
		})
	})

	Describe("removing all routes configurations managed by the gateway route manager", func() {
		Context("removing routes, should return nil", func() {
			JustBeforeEach(func() {
				existingRoutesGRM = setUpRoutes(routesGRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDGRM)
			})

			It("routes should be correctly removed", func() {
				err := grm.CleanRoutingTable()
				Expect(err).ShouldNot(HaveOccurred())
				// Try to list rules
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesGRM[0], netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
			})
		})

		Context("removing policy routing rules, should return nil", func() {
			JustBeforeEach(func() {
				existingRoutesGRM = setUpRoutes(routesGRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDGRM)
			})

			It("policy routing rules should be correctly removed", func() {
				err := grm.CleanPolicyRules()
				Expect(err).ShouldNot(HaveOccurred())
				// Try to list rules
				exists, err := existsRuleForRoutingTable(routingTableIDGRM)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(exists).Should(BeFalse())
			})
		})
	})
})
