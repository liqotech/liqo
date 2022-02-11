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
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"

	"github.com/liqotech/liqo/pkg/liqonet/errors"
)

var (
	dstNetCorrect                    = "10.244.0.0/16"
	srcNetCorrect                    = "10.200.0.0/16"
	gwIPCorrect                      = "10.0.0.5"
	dstNetWrong                      = "10.244.0.0.16"
	srcNetWrong                      = "10.200.00/16"
	gwIPWrong                        = "10.00.5"
	notReachableIP                   = "10.100.1.1"
	route                            *netlink.Route
	existingRuleTo, existingRuleFrom *netlink.Rule
	destinationNet                   *net.IPNet
	gatewayIP                        net.IP

	routesCM = []routingInfo{
		{
			destinationNet: "10.10.0.0/16",
			gatewayIP:      "10.0.0.10",
			iFaceIndex:     0,
			routingTableID: routingTableID,
		},
		{
			destinationNet: "10.11.0.0/16",
			gatewayIP:      "",
			iFaceIndex:     0,
			routingTableID: routingTableID,
		}}
	existingRoutesCM []*netlink.Route
)

var _ = Describe("Common", func() {
	BeforeEach(func() {
		var err error
		// Parse dstNetCorrect.
		_, destinationNet, err = net.ParseCIDR(dstNetCorrect)
		Expect(err).NotTo(HaveOccurred())
		route = &netlink.Route{Dst: destinationNet, Table: routingTableID}
		// Parse gwIPCorrect.
		gatewayIP = net.ParseIP(gwIPCorrect)
		Expect(gatewayIP).ShouldNot(BeNil())
		// Populate the index of the routes with the correct one.
		for i := range routesCM {
			routesCM[i].iFaceIndex = dummylink1.Attrs().Index
		}
	})

	Describe("adding new route", func() {
		Context("when input parameters are not in the correct format", func() {
			It("should return error on wrong destination net", func() {
				added, err := AddRoute(dstNetWrong, gwIPCorrect, dummylink1.Attrs().Index, routingTableID, DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(false))
				Expect(err).Should(Equal(&net.ParseError{Type: "CIDR address", Text: dstNetWrong}))
			})

			It("should return error on wrong gateway IP address", func() {
				added, err := AddRoute(dstNetCorrect, gwIPWrong, dummylink1.Attrs().Index, routingTableID, DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(false))
				Expect(err).Should(Equal(&errors.ParseIPError{IPToBeParsed: gwIPWrong}))
			})
		})

		Context("when an error occurred while adding a route", func() {
			It("should return an error on non existing link", func() {
				added, err := AddRoute(dstNetCorrect, gwIPWrong, 0, routingTableID, DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(false))
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when route does not exist and we want to add it", func() {
			It("no gatewayIP, should return true and nil", func() {
				added, err := AddRoute(dstNetCorrect, "", dummylink1.Attrs().Index, routingTableID, DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				// Get the route and check it has the right parameters
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, route, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).NotTo(HaveOccurred())
				Expect(routes[0].Gw).Should(BeNil())
			})

			It("with gatewayIP, should return true and nil", func() {
				added, err := AddRoute(dstNetCorrect, gwIPCorrect, dummylink1.Attrs().Index, routingTableID, DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				// Get the route and check it has the right parameters
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, route, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).NotTo(HaveOccurred())
				Expect(routes[0].Gw).Should(Equal(gatewayIP.To4()))
			})
		})

		Context("when route does exist and we want to add it", func() {
			JustBeforeEach(func() {
				existingRoutesCM = setUpRoutes(routesCM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableID)
			})

			It("should return false and nil", func() {
				// Add existing route with GW.
				added, err := AddRoute(existingRoutesCM[0].Dst.String(), existingRoutesCM[0].Gw.String(), existingRoutesCM[0].LinkIndex,
					existingRoutesCM[0].Table, DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(false))
				Expect(err).NotTo(HaveOccurred())

				// Add existing route without GW.
				added, err = AddRoute(existingRoutesCM[1].Dst.String(), "", existingRoutesCM[1].LinkIndex, existingRoutesCM[1].Table,
					DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(false))
				Expect(err).NotTo(HaveOccurred())
			})

			It("update gateway of existing route: should return true and nil", func() {
				// Update route with GW
				added, err := AddRoute(existingRoutesCM[0].Dst.String(), "", existingRoutesCM[0].LinkIndex, existingRoutesCM[0].Table,
					DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				// Get the route and check it has the right parameters
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesCM[0], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).NotTo(HaveOccurred())
				Expect(routes[0].Gw).Should(BeNil())

				// Update route without GW
				added, err = AddRoute(existingRoutesCM[1].Dst.String(), gwIPCorrect, existingRoutesCM[1].LinkIndex, existingRoutesCM[1].Table,
					DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				// Get the route and check it has the right parameters
				routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesCM[1], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).NotTo(HaveOccurred())
				Expect(routes[0].Gw.String()).Should(Equal(gwIPCorrect))
			})

			It("update link index of existing route: should return true and nil", func() {
				// Update route with GW
				added, err := AddRoute(existingRoutesCM[0].Dst.String(), existingRoutesCM[0].Gw.String(), dummyLink2.Attrs().Index,
					existingRoutesCM[0].Table, DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				// Get the route and check it has the right parameters
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesCM[0], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).NotTo(HaveOccurred())
				Expect(routes[0].LinkIndex).Should(BeNumerically("==", dummyLink2.Attrs().Index))

				// Update route without GW
				added, err = AddRoute(existingRoutesCM[1].Dst.String(), gwIPCorrect, dummyLink2.Attrs().Index,
					existingRoutesCM[1].Table, DefaultFlags, DefaultScope)
				Expect(added).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				// Get the route and check it has the right parameters
				routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesCM[1], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).NotTo(HaveOccurred())
				Expect(routes[0].LinkIndex).Should(BeNumerically("==", dummyLink2.Attrs().Index))
				Expect(routes[0].Gw.String()).Should(Equal(gwIPCorrect))
			})
		})
	})

	Describe("deleting an existing route", func() {
		Context("when input parameters are not in the correct format", func() {
			It("should return error on wrong destination net", func() {
				removed, err := DelRoute(dstNetWrong, gwIPCorrect, dummylink1.Attrs().Index, routingTableID)
				Expect(removed).Should(Equal(false))
				Expect(err).Should(Equal(&net.ParseError{Type: "CIDR address", Text: dstNetWrong}))
			})

			It("should return error on wrong gateway IP address", func() {
				removed, err := DelRoute(dstNetCorrect, gwIPWrong, dummylink1.Attrs().Index, routingTableID)
				Expect(removed).Should(Equal(false))
				Expect(err).Should(Equal(&errors.ParseIPError{IPToBeParsed: gwIPWrong}))
			})
		})

		Context("when an error occurred while deleting a route", func() {
			It("should return an error on non existing link", func() {
				added, err := DelRoute(dstNetCorrect, gwIPWrong, 0, routingTableID)
				Expect(added).Should(Equal(false))
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when route does not exist and we want to delete it", func() {
			It("no gatewayIP, should return false and nil", func() {
				removed, err := DelRoute(dstNetCorrect, "", dummylink1.Attrs().Index, routingTableID)
				Expect(removed).Should(Equal(false))
				Expect(err).NotTo(HaveOccurred())
			})

			It("with gatewayIP, should return false and nil", func() {
				removed, err := DelRoute(dstNetCorrect, gwIPCorrect, dummylink1.Attrs().Index, routingTableID)
				Expect(removed).Should(Equal(false))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when route does exist and we want to delete it", func() {
			JustBeforeEach(func() {
				existingRoutesCM = setUpRoutes(routesCM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableID)
			})

			It("with gateway, should return true and nil", func() {
				// Delete existing route with GW.
				removed, err := DelRoute(existingRoutesCM[0].Dst.String(), existingRoutesCM[0].Gw.String(), existingRoutesCM[0].LinkIndex, existingRoutesCM[0].Table)
				Expect(removed).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				// Expecting no routes exist for the given destination.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesCM[0], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(len(routes)).Should(BeNumerically("==", 0))
				Expect(err).NotTo(HaveOccurred())
			})

			It("without gateway, should return true and nil", func() {
				// Del existing route without GW.
				removed, err := DelRoute(existingRoutesCM[1].Dst.String(), "", existingRoutesCM[1].LinkIndex, existingRoutesCM[1].Table)
				Expect(removed).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				// Expecting no routes exist for the given destination.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesCM[1], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(len(routes)).Should(BeNumerically("==", 0))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("flushing custom routing table", func() {
		JustBeforeEach(func() {
			existingRoutesCM = setUpRoutes(routesCM)
		})

		JustAfterEach(func() {
			tearDownRoutes(routingTableID)
		})

		It("should remove all the routes from the custom table", func() {
			err := flushRoutesForRoutingTable(routingTableID)
			Expect(err).NotTo(HaveOccurred())
			// Check that the routing table is empty
			routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesCM[0], netlink.RT_FILTER_TABLE)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(routes)).Should(BeNumerically("==", 0))
		})
	})

	Describe("adding new policy routing rule", func() {
		AfterEach(func() {
			tearDownRules()
		})

		Context("when input parameters are not in the correct format", func() {
			It("should return error on wrong destination net", func() {
				added, err := AddPolicyRoutingRule(srcNetCorrect, dstNetWrong, routingTableID)
				Expect(added).Should(Equal(false))
				Expect(err).Should(Equal(&net.ParseError{Type: "CIDR address", Text: dstNetWrong}))
			})

			It("should return error on wrong source net", func() {
				added, err := AddPolicyRoutingRule(srcNetWrong, dstNetCorrect, routingTableID)
				Expect(added).Should(Equal(false))
				Expect(err).Should(Equal(&net.ParseError{Type: "CIDR address", Text: srcNetWrong}))
			})

			It("should return error if both subnets are empty", func() {
				added, err := AddPolicyRoutingRule("", "", routingTableID)
				Expect(added).Should(Equal(false))
				Expect(err).Should(Equal(&errors.WrongParameter{
					Parameter: "fromSubnet and toSubnet",
					Reason:    errors.AtLeastOneValid,
				}))
			})
		})
	})

	Context("when policy routing rule does not exist and we want to add it", func() {
		It("only to destination net, should return true and nil", func() {
			added, err := AddPolicyRoutingRule("", dstNetCorrect, routingTableID)
			Expect(added).Should(Equal(true))
			Expect(err).NotTo(HaveOccurred())
			// Get the rule and check it has the right parameters.
			rule, err := getRule("", dstNetCorrect, routingTableID)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Dst.String()).Should(Equal(dstNetCorrect))
			Expect(rule.Src).Should(BeNil())
		})

		It("only to source net, should return true and nil", func() {
			added, err := AddPolicyRoutingRule(srcNetCorrect, "", routingTableID)
			Expect(added).Should(Equal(true))
			Expect(err).NotTo(HaveOccurred())
			// Get the rule and check it has the right parameters.
			rule, err := getRule(srcNetCorrect, "", routingTableID)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Src.String()).Should(Equal(srcNetCorrect))
			Expect(rule.Dst).Should(BeNil())
		})

		It("both source and destination net, should return true and nil", func() {
			added, err := AddPolicyRoutingRule(srcNetCorrect, dstNetCorrect, routingTableID)
			Expect(added).Should(Equal(true))
			Expect(err).NotTo(HaveOccurred())
			// Get the rule and check it has the right parameters.
			rule, err := getRule(srcNetCorrect, dstNetCorrect, routingTableID)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Src.String()).Should(Equal(srcNetCorrect))
			Expect(rule.Dst.String()).Should(Equal(dstNetCorrect))
		})
	})

	Context("when policy routing rule does exist and we want to add it", func() {
		BeforeEach(func() {
			setUpRules()
		})

		AfterEach(func() {
			tearDownRules()
		})
		It("rule already exists: should return false and nil", func() {
			added, err := AddPolicyRoutingRule(existingRuleFrom.Src.String(), "", routingTableID)
			Expect(added).Should(Equal(false))
			Expect(err).NotTo(HaveOccurred())
			// Get the rule and check it has the right parameters.
			rule, err := getRule(existingRuleFrom.Src.String(), "", routingTableID)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Src.String()).Should(Equal(existingRuleFrom.Src.String()))
			Expect(rule.Dst).Should(BeNil())
		})

		It("update routing table ID: should return true and nil", func() {
			routingTable := 12345
			added, err := AddPolicyRoutingRule(existingRuleFrom.Src.String(), "", routingTable)
			Expect(added).Should(Equal(true))
			Expect(err).NotTo(HaveOccurred())
			// Get the rule and check it has the right parameters.
			rule, err := getRule(existingRuleFrom.Src.String(), "", routingTable)
			Expect(err).NotTo(HaveOccurred())
			Expect(rule.Src.String()).Should(Equal(existingRuleFrom.Src.String()))
			Expect(rule.Dst).Should(BeNil())
		})
	})

	Describe("deleting an existing policy routing rule", func() {
		Context("when input parameters are not in the correct format", func() {
			It("should return error on wrong destination net", func() {
				removed, err := DelPolicyRoutingRule(dstNetWrong, "", routingTableID)
				Expect(removed).Should(Equal(false))
				Expect(err).Should(Equal(&net.ParseError{Type: "CIDR address", Text: dstNetWrong}))
			})

			It("should return error on wrong source net", func() {
				removed, err := DelPolicyRoutingRule("", srcNetWrong, routingTableID)
				Expect(removed).Should(Equal(false))
				Expect(err).Should(Equal(&net.ParseError{Type: "CIDR address", Text: srcNetWrong}))
			})

			It("should return error if both subnets are empty", func() {
				removed, err := DelPolicyRoutingRule("", "", routingTableID)
				Expect(removed).Should(Equal(false))
				Expect(err).Should(Equal(&errors.WrongParameter{
					Parameter: "fromSubnet and toSubnet",
					Reason:    errors.AtLeastOneValid,
				}))
			})
		})

		Context("when policy routing rule does not exist and we want to delete it", func() {
			It("with destination net, should return false and nil", func() {
				removed, err := DelPolicyRoutingRule(dstNetCorrect, "", routingTableID)
				Expect(removed).Should(Equal(false))
				Expect(err).NotTo(HaveOccurred())
			})

			It("with source net, should return false and nil", func() {
				removed, err := DelPolicyRoutingRule("", srcNetCorrect, routingTableID)
				Expect(removed).Should(Equal(false))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when policy routing rule does exist and we want to delete it", func() {
			JustBeforeEach(func() {
				setUpRules()
			})

			JustAfterEach(func() {
				tearDownRules()
			})

			It("with destination net, should return false and nil", func() {
				removed, err := DelPolicyRoutingRule("", existingRuleTo.Dst.String(), routingTableID)
				Expect(removed).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				rule, err := getRule("", existingRuleTo.Dst.String(), routingTableID)
				Expect(rule).To(BeNil())
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).Should(Equal("rule not found"))
			})

			It("with source net, should return false and nil", func() {
				removed, err := DelPolicyRoutingRule(existingRuleFrom.Src.String(), "", routingTableID)
				Expect(removed).Should(Equal(true))
				Expect(err).NotTo(HaveOccurred())
				rule, err := getRule("", existingRuleFrom.Src.String(), routingTableID)
				Expect(rule).To(BeNil())
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).Should(Equal("rule not found"))
			})
		})
	})

	Describe("flushing policy routing rules for a custom routing table", func() {
		JustBeforeEach(func() {
			setUpRules()
		})

		JustAfterEach(func() {
			tearDownRules()
		})

		It("should remove all the policy routing rules referencing the custom table", func() {
			err := flushRulesForRoutingTable(routingTableID)
			Expect(err).NotTo(HaveOccurred())
			// Check that no policy routing rules reference the custom routing table.
			exists, err := existsRuleForRoutingTable(routingTableID)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).Should(BeFalse())
		})
	})

	Describe("getting interface index from which a given IP address is reachable", func() {

		Context("when input parameters are not in the correct format", func() {
			It("should return error on malformed IP address", func() {
				index, err := getIFaceIndexForIP(gwIPWrong)
				Expect(index).To(BeZero())
				Expect(err).Should(Equal(&errors.ParseIPError{IPToBeParsed: gwIPWrong}))
			})
		})

		Context("when there is a route for the given IP address", func() {
			It("should return nil and a correct link index", func() {
				index, err := getIFaceIndexForIP(gwIPCorrect)
				Expect(err).NotTo(HaveOccurred())
				Expect(index).Should(BeNumerically("==", dummylink1.Attrs().Index))
			})
		})

		Context("when there is no route for the given IP address", func() {
			It("should return error and 0 as link index", func() {
				index, err := getIFaceIndexForIP(notReachableIP)
				Expect(err).To(HaveOccurred())
				Expect(index).Should(BeZero())
				Expect(err).Should(Equal(&errors.NoRouteFound{IPAddress: notReachableIP}))
			})
		})
	})

	Describe("getting routing information from a tunnelendpoint instance", func() {

		Context("when the the operator has same IP address as the Gateway pod", func() {
			It("should return no error", func() {
				dstPodCIDRNet, dstExternalCIDRNet, gwIP, iFaceIndex, err := getRouteConfig(&tep, ipAddress2NoSubnet)
				Expect(err).NotTo(HaveOccurred())
				Expect(dstPodCIDRNet).Should(Equal(tep.Spec.RemoteNATPodCIDR))
				Expect(dstExternalCIDRNet).Should(Equal(tep.Spec.RemoteNATExternalCIDR))
				Expect(gwIP).Should(Equal(""))
				Expect(iFaceIndex).Should(BeNumerically("==", tep.Status.VethIFaceIndex))
			})
		})

		Context("when the the operator is not running on same node as the Gateway pod", func() {
			It("should return nil and a link index of the interface through which the Gateway is reachable", func() {
				dstPodCIDRNet, dstExternalCIDRNet, gwIP, iFaceIndex, err := getRouteConfig(&tep, notReachableIP)
				Expect(err).NotTo(HaveOccurred())
				Expect(dstPodCIDRNet).Should(Equal(tep.Spec.RemoteNATPodCIDR))
				Expect(dstExternalCIDRNet).Should(Equal(tep.Spec.RemoteNATExternalCIDR))
				Expect(gwIP).Should(Equal(ipAddress2NoSubnet))
				Expect(iFaceIndex).Should(BeNumerically("==", dummylink1.Attrs().Index))
			})
		})

		Context("when the gateway IP address is not reachable", func() {
			It("should return error", func() {
				tepCopy := tep
				tepCopy.Status.GatewayIP = notReachableIP
				_, _, _, _, err := getRouteConfig(&tepCopy, gwIPCorrect)
				Expect(err).To(HaveOccurred())
				Expect(err).Should(Equal(&errors.NoRouteFound{IPAddress: tepCopy.Status.GatewayIP}))
			})
		})
	})

	Describe("enabling ip forwarding for ipv4", func() {
		Context("enable ip forwarding", func() {
			It("should return nil", func() {
				var enabled byte = '1'
				err := EnableIPForwarding()
				Expect(err).ShouldNot(HaveOccurred())
				txt, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
				Expect(err).ShouldNot(HaveOccurred())
				Expect(txt[0]).Should(Equal(enabled))
			})
		})
	})
})
