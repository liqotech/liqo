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

package routeoperator

import (
	"net"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqonet/overlay"
)

var (
	vxlanConfig = &overlay.VxlanDeviceAttrs{
		Vni:      1800,
		Name:     "vxlan.route",
		VtepPort: 4789,
		VtepAddr: nil,
		MTU:      1450,
	}
	vxlanDevice   = new(overlay.VxlanDevice)
	vxlanDeviceIP = "240.0.0.1/8"
	k8sClient     client.Client
)

func TestRouteOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RouteOperator Suite")
}

var _ = BeforeSuite(func() {
	/*** Common setup ***/
	link, err := setUpVxlanLink(vxlanConfig)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(link).ShouldNot(BeNil())
	vxlanDevice.Link = link.(*netlink.Vxlan)
	Expect(vxlanDevice.ConfigureIPAddress(vxlanDeviceIP)).To(BeNil())

	/*** OverlayOperator configuration ***/
	// Configure existing neigh.
	peerIP := net.ParseIP(overlayPeerIP)
	Expect(peerIP).NotTo(BeNil())
	peerMAC, err := net.ParseMAC(overlayPeerMAC)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(peerMAC).NotTo(BeNil())
	overlayExistingNeigh.IP = peerIP
	overlayExistingNeigh.MAC = peerMAC
	macZeros, err := net.ParseMAC("00:00:00:00:00:00")
	Expect(err).To(BeNil())
	overlayExistingNeighDef.IP = peerIP
	overlayExistingNeighDef.MAC = macZeros
	// Configure neigh.
	peerIP1 := net.ParseIP(overlayPodIP)
	Expect(peerIP).NotTo(BeNil())
	peerMAC, err = net.ParseMAC(overlayAnnValue)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(peerMAC).NotTo(BeNil())
	overlayNeigh.IP = peerIP1
	overlayNeigh.MAC = peerMAC

	/*** Symmetric Routing operator configuration ***/

	// Setup envtest
	Expect(setupOverlayTestEnv()).To(BeNil())
})

var _ = AfterSuite(func() {
	Expect(netlink.LinkDel(vxlanDevice.Link)).ShouldNot(HaveOccurred())
})

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

	return link, nil
}
