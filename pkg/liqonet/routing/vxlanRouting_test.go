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
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqoerrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
)

var (
	routingTableIDVRM = 19953
	overlayDevIP      = "240.1.1.1/8"
	overlayNetPrexif  = liqoconst.OverlayNetPrefix
	routesVRM         = []routingInfo{
		{
			destinationNet: "10.110.0.0/16",
			gatewayIP:      "240.0.0.100",
			iFaceIndex:     0,
			routingTableID: routingTableIDVRM,
		},
		{
			destinationNet: "10.111.0.0/16",
			gatewayIP:      "240.0.0.100",
			iFaceIndex:     0,
			routingTableID: routingTableIDVRM,
		}}
	vxlanConfig = &overlay.VxlanDeviceAttrs{
		Vni:      18952,
		Name:     "vxlan.test",
		VtepPort: 4789,
		VtepAddr: nil,
		MTU:      1450,
	}
	tepVRM            netv1alpha1.TunnelEndpoint
	existingRoutesVRM []*netlink.Route
	overlayDevice     *overlay.VxlanDevice
)

func setUpVxlanLink(attrs *overlay.VxlanDeviceAttrs) (netlink.Link, error) {
	err := netlink.LinkAdd(&netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:  attrs.Name,
			MTU:   attrs.MTU,
			Flags: net.FlagUp,
		},
		VxlanId:  attrs.Vni,
		SrcAddr:  attrs.VtepAddr,
		Port:     attrs.VtepPort,
		Learning: true,
	})
	if err != nil {
		return nil, err
	}

	link, err := netlink.LinkByName(attrs.Name)
	if err != nil {
		return nil, err
	}
	// Parse ip.
	vxlanIp, vxlanIpNet, err := net.ParseCIDR(overlayDevIP)
	if err != nil {
		return nil, err
	}

	// Add ip to the vxlan link.
	err = netlink.AddrAdd(link, &netlink.Addr{IPNet: &net.IPNet{
		IP:   vxlanIp,
		Mask: vxlanIpNet.Mask,
	}})
	if err != nil {
		return nil, err
	}
	return link, nil
}

func deleteLink(name string) error {
	// Get link by name
	link, err := netlink.LinkByName(name)
	if err != nil && err.Error() != ("Link "+name+" not found") {
		return err
	}
	if err == nil {
		return netlink.LinkDel(link)
	}
	return nil
}

var _ = Describe("VxlanRouting", func() {
	BeforeEach(func() {
		// Populate the index of the routes with the correct one.
		for i := range routesVRM {
			routesVRM[i].iFaceIndex = overlayDevice.Link.Attrs().Index
		}
	})
	Describe("creating new Vxlan Route Manager", func() {

		Context("when parameters are not valid", func() {
			It("routingTableID parameter out of range: a negative number", func() {
				vrm, err := NewVxlanRoutingManager(-244, gwIPCorrect, overlayNetPrexif, overlayDevice)
				Expect(vrm).Should(BeNil())
				Expect(err).Should(Equal(&liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.GreaterOrEqual + strconv.Itoa(0)}))
			})

			It("routingTableID parameter out of range: superior to max value ", func() {
				vrm, err := NewVxlanRoutingManager(unix.RT_TABLE_MAX+1, gwIPCorrect, overlayNetPrexif, overlayDevice)
				Expect(vrm).Should(BeNil())
				Expect(err).Should(Equal(&liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.MinorOrEqual + strconv.Itoa(unix.RT_TABLE_MAX)}))
			})

			It("podIP is not in right format", func() {
				vrm, err := NewVxlanRoutingManager(244, gwIPWrong, overlayNetPrexif, overlayDevice)
				Expect(vrm).Should(BeNil())
				Expect(err).Should(Equal(&liqoerrors.ParseIPError{IPToBeParsed: gwIPWrong}))
			})

			It("vxlan link is nil", func() {
				vrm, err := NewVxlanRoutingManager(244, gwIPCorrect, overlayNetPrexif, nil)
				Expect(vrm).Should(BeNil())
				Expect(err).Should(Equal(&liqoerrors.WrongParameter{Parameter: "vxlanDevice", Reason: liqoerrors.NotNil}))
			})

			It("vxlanNetPrefix is empty", func() {
				vrm, err := NewVxlanRoutingManager(244, gwIPCorrect, "", overlayDevice)
				Expect(vrm).Should(BeNil())
				Expect(err).Should(Equal(&liqoerrors.WrongParameter{Parameter: "vxlanNetPrefix", Reason: liqoerrors.StringNotEmpty}))
			})
		})

		Context("when parameters are correct", func() {
			It("right parameters", func() {
				vrm, err := NewVxlanRoutingManager(244, gwIPCorrect, overlayNetPrexif, overlayDevice)
				Expect(vrm).ShouldNot(BeNil())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("configuring routes for a remote peering cluster", func() {
		JustBeforeEach(func() {
			tepVRM = netv1alpha1.TunnelEndpoint{
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
			It("route configuration fails while adding policy routing rule for PodCIDR", func() {
				tepVRM.Spec.RemoteNATPodCIDR = ""
				added, err := vrm.EnsureRoutesPerCluster(&tepVRM)
				Expect(added).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})

			It("route configuration fails while adding policy routing rule for ExternalCIDR", func() {
				tepVRM.Spec.RemoteNATExternalCIDR = ""
				added, err := vrm.EnsureRoutesPerCluster(&tepVRM)
				Expect(added).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})

			It("route configuration fails while adding route", func() {
				tepVRM.Status.GatewayIP = ipAddress1NoSubnet
				added, err := vrm.EnsureRoutesPerCluster(&tepVRM)
				Expect(added).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Context("when tep holds correct parameters", func() {
		JustBeforeEach(func() {
			existingRoutesVRM = setUpRoutes(routesVRM)
		})

		JustAfterEach(func() {
			tearDownRoutes(routingTableIDVRM)
		})

		It("route configuration should be correctly inserted", func() {
			added, err := vrm.EnsureRoutesPerCluster(&tep)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(added).Should(BeTrue())
			// Get inserted routes
			_, dstPodCIDRNet, err := net.ParseCIDR(tep.Spec.RemoteNATPodCIDR)
			Expect(err).ShouldNot(HaveOccurred())
			_, dstExternalCIDRNet, err := net.ParseCIDR(tep.Spec.RemoteNATExternalCIDR)
			Expect(err).ShouldNot(HaveOccurred())
			routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: dstPodCIDRNet,
				Table: routingTableIDVRM}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(routes[0].Dst.String()).Should(Equal(tep.Spec.RemoteNATPodCIDR))
			Expect(routes[0].Gw.String()).Should(Equal(ipAddress2NoSubnetOverlay))
			routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: dstExternalCIDRNet,
				Table: routingTableIDVRM}, netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(routes[0].Dst.String()).Should(Equal(tep.Spec.RemoteNATExternalCIDR))
			Expect(routes[0].Gw.String()).Should(Equal(ipAddress2NoSubnetOverlay))
		})

		It("routes already exist, should return false and nil", func() {
			tepVRM.Spec.RemoteNATPodCIDR = existingRoutesVRM[0].Dst.String()
			tepVRM.Status.GatewayIP = existingRoutesVRM[0].Gw.String()
			tepVRM.Spec.RemoteNATExternalCIDR = existingRoutesVRM[1].Dst.String()
			added, err := vrm.EnsureRoutesPerCluster(&tepVRM)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(added).Should(BeFalse())
		})

		It("route is outdated, should return true and nil", func() {
			tepVRM.Spec.RemoteNATPodCIDR = existingRoutesVRM[1].Dst.String()
			tepVRM.Status.GatewayIP = gwIPCorrect
			added, err := vrm.EnsureRoutesPerCluster(&tepVRM)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(added).Should(BeTrue())
			// Check that the route has been updated.
			routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesVRM[1], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(routes[0].Dst.String()).Should(Equal(tepVRM.Spec.RemoteNATPodCIDR))
			Expect(routes[0].Gw.String()).Should(Equal("240.0.0.5"))
		})
	})

	Describe("removing route configuration for a remote peering cluster", func() {
		Context("when tep holds malformed parameters", func() {
			It("fails to remove route configuration while removing policy routing rule for PodCIDR", func() {
				tepVRM.Spec.RemoteNATPodCIDR = ""
				added, err := vrm.RemoveRoutesPerCluster(&tepVRM)
				Expect(added).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
			It("fails to remove route configuration while removing policy routing rule for ExternalCIDR", func() {
				tepVRM.Spec.RemoteNATExternalCIDR = ""
				added, err := vrm.RemoveRoutesPerCluster(&tepVRM)
				Expect(added).Should(BeFalse())
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when tep holds correct parameters", func() {
			JustBeforeEach(func() {
				existingRoutesVRM = setUpRoutes(routesVRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDVRM)
			})

			It("route configuration should be correctly removed", func() {
				tepVRM.Spec.RemoteNATPodCIDR = existingRoutesVRM[0].Dst.String()
				tepVRM.Spec.RemoteNATExternalCIDR = existingRoutesVRM[1].Dst.String()
				tepVRM.Status.GatewayIP = existingRoutesVRM[1].Gw.String()
				added, err := vrm.RemoveRoutesPerCluster(&tepVRM)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeTrue())
				// Try to get the remove route.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesVRM[0], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
				routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesVRM[1], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
			})

			It("route configuration should be correctly removed from veth pair", func() {
				tepVRM.Spec.RemoteNATPodCIDR = existingRoutesVRM[1].Dst.String()
				tepVRM.Spec.RemoteNATExternalCIDR = existingRoutesVRM[0].Dst.String()
				tepVRM.Status.GatewayIP = ipAddress1NoSubnet
				tepVRM.Status.VethIP = existingRoutesVRM[1].Gw.String()
				tepVRM.Status.VethIFaceIndex = overlayDevice.Link.Index
				added, err := vrm.RemoveRoutesPerCluster(&tepVRM)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeTrue())
				// Try to get the remove route.
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesVRM[1], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
				routes, err = netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesVRM[0], netlink.RT_FILTER_DST|netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
			})

			It("route does not exist, should return false and nil", func() {
				added, err := vrm.RemoveRoutesPerCluster(&tep)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeFalse())
			})
		})
	})

	Describe("removing all routes configurations managed by the vxlan route manager", func() {
		Context("removing routes, should return nil", func() {
			JustBeforeEach(func() {
				existingRoutesVRM = setUpRoutes(routesVRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDVRM)
			})

			It("routes should be correctly removed", func() {
				err := vrm.CleanRoutingTable()
				Expect(err).ShouldNot(HaveOccurred())
				// Try to list rules
				routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRoutesVRM[0], netlink.RT_FILTER_TABLE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(routes)).Should(BeZero())
			})
		})

		Context("removing policy routing rules, should return nil", func() {
			JustBeforeEach(func() {
				existingRoutesVRM = setUpRoutes(routesVRM)
			})

			JustAfterEach(func() {
				tearDownRoutes(routingTableIDVRM)
			})

			It("policy routing rules should be correctly removed", func() {
				err := vrm.CleanPolicyRules()
				Expect(err).ShouldNot(HaveOccurred())
				// Try to list rules
				exists, err := existsRuleForRoutingTable(routingTableIDVRM)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(exists).Should(BeFalse())
			})
		})
	})
})
