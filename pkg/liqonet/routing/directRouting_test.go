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

	"github.com/liqotech/liqo/pkg/liqonet/errors"
)

var (
	routingTableIDDRM = 18953
	routesDRM         = []routingInfo{
		{
			destinationNet: "10.100.0.0/16",
			gatewayIP:      "10.0.0.100",
			iFaceIndex:     0,
			routingTableID: routingTableIDDRM,
		},
		{
			destinationNet: "10.101.0.0/16",
			gatewayIP:      "10.0.0.100",
			iFaceIndex:     0,
			routingTableID: routingTableIDDRM,
		}}
	existingRoutesDRM []*netlink.Route
)

var _ = Describe("DirectRouting", func() {
	BeforeEach(func() {
		// Populate the index of the routes with the correct one.
		for i := range routesDRM {
			routesDRM[i].iFaceIndex = dummylink1.Attrs().Index
		}
	})
	Describe("creating new Direct Route Manager", func() {

		Context("when parameters are not valid", func() {
			It("routingTableID parameter out of range: a negative number", func() {
				drm, err := NewDirectRoutingManager(-244, gwIPCorrect)
				Expect(drm).Should(BeNil())
				Expect(err).Should(Equal(&errors.WrongParameter{Parameter: "routingTableID", Reason: errors.GreaterOrEqual + strconv.Itoa(0)}))
			})

			It("routingTableID parameter out of range: superior to max value ", func() {
				drm, err := NewDirectRoutingManager(unix.RT_TABLE_MAX+1, gwIPCorrect)
				Expect(drm).Should(BeNil())
				Expect(err).Should(Equal(&errors.WrongParameter{Parameter: "routingTableID", Reason: errors.MinorOrEqual + strconv.Itoa(unix.RT_TABLE_MAX)}))
			})

			It("podIP is not in right format", func() {
				drm, err := NewDirectRoutingManager(244, gwIPWrong)
				Expect(drm).Should(BeNil())
				Expect(err).Should(Equal(&errors.ParseIPError{IPToBeParsed: gwIPWrong}))
			})
		})

		Context("when parameters are correct", func() {
			It("right parameters", func() {
				drm, err := NewDirectRoutingManager(244, gwIPCorrect)
				Expect(drm).ShouldNot(BeNil())
				Expect(err).Should(BeNil())
			})
		})
	})

	Describe("configuring routes for a remote peering cluster", func() {
		Context("when tep holds malformed parameters", func() {
			It("route configuration fails while extracting route information from tep", func() {
				tepCopy := tep
				tepCopy.Status.GatewayIP = notReachableIP
				added, err := drm.EnsureRoutesPerCluster(&tepCopy)
				Expect(err).Should(Equal(&errors.NoRouteFound{IPAddress: tepCopy.Status.GatewayIP}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})

			It("route configuration fails while adding policy routing rule for PodCIDR", func() {
				tepCopy := tep
				tepCopy.Spec.RemoteNATPodCIDR = ""
				added, err := drm.EnsureRoutesPerCluster(&tepCopy)
				Expect(err).Should(Equal(&errors.WrongParameter{
					Parameter: "fromSubnet and toSubnet",
					Reason:    errors.AtLeastOneValid,
				}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})

			It("route configuration fails while adding policy routing rule for ExternalCIDR", func() {
				tepCopy := tep
				tepCopy.Spec.RemoteNATExternalCIDR = ""
				added, err := drm.EnsureRoutesPerCluster(&tepCopy)
				Expect(err).Should(Equal(&errors.WrongParameter{
					Parameter: "fromSubnet and toSubnet",
					Reason:    errors.AtLeastOneValid,
				}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})

			It("route configuration fails while adding route", func() {
				tepCopy := tep
				tepCopy.Status.GatewayIP = ipAddress1NoSubnet
				added, err := drm.EnsureRoutesPerCluster(&tepCopy)
				Expect(err).Should(Equal(unix.ENODEV))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when tep holds correct parameters", func() {
			JustBeforeEach(func() {
				existingRoutesDRM = setUpRoutes(routesDRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDDRM)
			})

			It("route configuration should be correctly inserted", func() {
				added, err := drm.EnsureRoutesPerCluster(&tep)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeTrue())
				// Get the inserted route
				_, dstPodCIDRNet, err := net.ParseCIDR(tep.Spec.RemoteNATPodCIDR)
				Expect(err).ShouldNot(HaveOccurred())
				_, dstExternalCIDRNet, err := net.ParseCIDR(tep.Spec.RemoteNATExternalCIDR)
				Expect(err).ShouldNot(HaveOccurred())
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: dstPodCIDRNet,
					Table: routingTableIDDRM}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(routes[0].Dst.String()).Should(Equal(tep.Spec.RemoteNATPodCIDR))
				Expect(routes[0].Gw.String()).Should(Equal(tep.Status.GatewayIP))
				routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: dstExternalCIDRNet,
					Table: routingTableIDDRM}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(routes[0].Dst.String()).Should(Equal(tep.Spec.RemoteNATExternalCIDR))
				Expect(routes[0].Gw.String()).Should(Equal(tep.Status.GatewayIP))
			})

			It("routes already exist, should return false and nil", func() {
				tepCopy := tep
				tepCopy.Spec.RemoteNATPodCIDR = existingRoutesDRM[0].Dst.String()
				tepCopy.Spec.RemoteNATExternalCIDR = existingRoutesDRM[1].Dst.String()
				tepCopy.Status.GatewayIP = existingRoutesDRM[0].Gw.String()
				existingRoutesDRM[1].Gw = existingRoutesDRM[0].Gw
				added, err := drm.EnsureRoutesPerCluster(&tepCopy)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeFalse())
			})

			It("route is outdated, should return true and nil", func() {
				tepCopy := tep
				tepCopy.Spec.RemoteNATPodCIDR = existingRoutesDRM[1].Dst.String()
				tepCopy.Status.GatewayIP = gwIPCorrect
				added, err := drm.EnsureRoutesPerCluster(&tepCopy)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeTrue())
				// Check that the route has been updated.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesDRM[1],
					netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(routes[0].Dst.String()).Should(Equal(tepCopy.Spec.RemoteNATPodCIDR))
				Expect(routes[0].Gw.String()).Should(Equal(tepCopy.Status.GatewayIP))
			})
		})
	})

	Describe("removing route configuration for a remote peering cluster", func() {
		Context("when tep holds malformed parameters", func() {
			It("fails to remove route configuration while extracting route information from tep", func() {
				tepCopy := tep
				tepCopy.Status.GatewayIP = notReachableIP
				added, err := drm.RemoveRoutesPerCluster(&tepCopy)
				Expect(err).Should(Equal(&errors.NoRouteFound{IPAddress: tepCopy.Status.GatewayIP}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})

			It("fails to remove route configuration while removing policy routing rule for PodCIDR", func() {
				tepCopy := tep
				tepCopy.Spec.RemoteNATPodCIDR = ""
				added, err := drm.RemoveRoutesPerCluster(&tepCopy)
				Expect(err).Should(Equal(&errors.WrongParameter{
					Parameter: "fromSubnet and toSubnet",
					Reason:    errors.AtLeastOneValid,
				}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})

			It("fails to remove route configuration while removing policy routing rule for ExternalCIDR", func() {
				tepCopy := tep
				tepCopy.Spec.RemoteNATExternalCIDR = ""
				added, err := drm.RemoveRoutesPerCluster(&tepCopy)
				Expect(err).Should(Equal(&errors.WrongParameter{
					Parameter: "fromSubnet and toSubnet",
					Reason:    errors.AtLeastOneValid,
				}))
				Expect(added).Should(BeFalse())
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when tep holds correct parameters", func() {
			JustBeforeEach(func() {
				existingRoutesDRM = setUpRoutes(routesDRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDDRM)
			})

			It("route configuration should be correctly removed", func() {
				tepCopy := tep
				tepCopy.Spec.RemoteNATPodCIDR = existingRoutesDRM[0].Dst.String()
				tepCopy.Spec.RemoteNATExternalCIDR = existingRoutesDRM[1].Dst.String()
				tepCopy.Status.GatewayIP = existingRoutesDRM[0].Gw.String()
				added, err := drm.RemoveRoutesPerCluster(&tepCopy)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeTrue())
				// Try to get removed routes.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesDRM[0], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
				routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesDRM[1], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
			})

			It("route does not exist, should return false and nil", func() {
				added, err := drm.RemoveRoutesPerCluster(&tep)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeFalse())
			})
		})
	})

	Describe("removing all routes configurations managed by the direct route manager", func() {
		Context("removing routes, should return nil", func() {
			JustBeforeEach(func() {
				existingRoutesDRM = setUpRoutes(routesDRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDDRM)
			})

			It("routes should be correctly removed", func() {
				err := drm.CleanRoutingTable()
				Expect(err).ShouldNot(HaveOccurred())
				// Try to list rules
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesDRM[0], netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
			})
		})

		Context("removing policy routing rules, should return nil", func() {
			JustBeforeEach(func() {
				existingRoutesDRM = setUpRoutes(routesDRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDDRM)
			})

			It("policy routing rules should be correctly removed", func() {
				err := drm.CleanPolicyRules()
				Expect(err).ShouldNot(HaveOccurred())
				// Try to list rules
				exists, err := existsRuleForRoutingTable(routingTableIDDRM)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(exists).Should(BeFalse())
			})
		})
	})
})
