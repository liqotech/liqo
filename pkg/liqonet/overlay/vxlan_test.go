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
	"net"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
)

var (
	vxlanConfig, vxlanOld *VxlanDeviceAttrs
	vxlanOldLink          netlink.Link
	vxlanDev              VxlanDevice
	fdbOld, fdbNew        Neighbor
)

func createLink(attrs *VxlanDeviceAttrs) (netlink.Link, error) {
	err := netlink.LinkAdd(&netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:  vxlanOld.Name,
			MTU:   vxlanOld.MTU,
			Flags: net.FlagUp,
		},
		VxlanId:  vxlanOld.Vni,
		SrcAddr:  vxlanOld.VtepAddr,
		Port:     vxlanOld.VtepPort,
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

func addFdb(neighbor Neighbor, ifaceIndex int) error {
	return netlink.NeighAdd(&netlink.Neigh{
		LinkIndex:    ifaceIndex,
		State:        netlink.NUD_PERMANENT | netlink.NUD_NOARP,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		Type:         netlink.NDA_DST,
		IP:           neighbor.IP,
		HardwareAddr: neighbor.MAC,
	})
}

func containsFdbEntry(fdbs []netlink.Neigh, n Neighbor) bool {
	for _, f := range fdbs {
		if f.IP.Equal(n.IP) && f.HardwareAddr.String() == n.MAC.String() {
			return true
		}
	}
	return false
}

func checkConfig(vxlanDev *VxlanDevice, vxlanOld *VxlanDeviceAttrs) {
	Expect(vxlanDev.Link.VxlanId).Should(BeNumerically("==", vxlanOld.Vni))
	Expect(vxlanDev.Link.MTU).Should(BeNumerically("==", vxlanOld.MTU))
	Expect(vxlanDev.Link.Port).Should(BeNumerically("==", vxlanOld.VtepPort))
	Expect(vxlanDev.Link.SrcAddr).Should(Equal(vxlanOld.VtepAddr))
	Expect(vxlanDev.Link.Name).Should(Equal(vxlanOld.Name))
}

var _ = Describe("Vxlan", func() {
	JustBeforeEach(func() {
		var err error
		vxlanConfig = &VxlanDeviceAttrs{
			Vni:      18952,
			Name:     "vxlan.test",
			VtepPort: 4789,
			VtepAddr: defaultIfaceIP,
			MTU:      1450,
		}
		vxlanOld = &VxlanDeviceAttrs{
			Vni:      18953,
			Name:     "vxlan.old",
			VtepAddr: defaultIfaceIP,
			VtepPort: 4789,
			MTU:      1450,
		}
		vxlanOldLink, err = createLink(vxlanOld)
		vxlanDev = VxlanDevice{Link: vxlanOldLink.(*netlink.Vxlan)}
		Expect(err).ShouldNot(HaveOccurred())
		Expect(vxlanOldLink).NotTo(BeNil())
		mac, err := net.ParseMAC("92:ce:cb:3b:82:ee")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(mac).ShouldNot(BeNil())
		ip := net.ParseIP("10.200.250.1")
		Expect(ip).ShouldNot(BeNil())
		fdbOld = Neighbor{MAC: mac, IP: ip}
		Expect(addFdb(fdbOld, vxlanOldLink.Attrs().Index)).ShouldNot(HaveOccurred())
		mac, err = net.ParseMAC("22:28:4f:ce:93:5d")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(mac).ShouldNot(BeNil())
		ip = net.ParseIP("10.200.250.2")
		Expect(ip).ShouldNot(BeNil())
		fdbNew = Neighbor{MAC: mac, IP: ip}
	})

	JustAfterEach(func() {
		Expect(deleteLink(vxlanOld.Name)).ShouldNot(HaveOccurred())
		vxlanOldLink = nil
	})
	Describe("creating a new vxlan device", func() {
		Context("the vxlan device does not exist", func() {
			It("should create the vxlan device, return the device and no error", func() {
				vxlanDev, err := NewVxlanDevice(vxlanConfig)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(vxlanDev).ShouldNot(BeNil())
				checkConfig(vxlanDev, vxlanConfig)
				Expect(deleteLink(vxlanConfig.Name)).ShouldNot(HaveOccurred())
			})

			It("should return an error because of the incorrect device name", func() {
				vxlanConfig.Name = "nameIsLongerThanFifteenBytes"
				vxlanDev, err := NewVxlanDevice(vxlanConfig)
				Expect(err).Should(HaveOccurred())
				Expect(err).Should(MatchError("failed to create the vxlan interface: numerical result out of range"))
				Expect(vxlanDev).Should(BeNil())

			})
		})

		Context("the vxlan device does exist with the same name", func() {
			It("should return the same device", func() {
				vxlanDev, err := NewVxlanDevice(vxlanOld)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(vxlanDev).ShouldNot(BeNil())
				checkConfig(vxlanDev, vxlanOld)
			})

			It("changing the source address, should return a new device", func() {
				vxlanOld.VtepAddr = nil
				vxlanDev, err := NewVxlanDevice(vxlanOld)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(vxlanDev).ShouldNot(BeNil())
				checkConfig(vxlanDev, vxlanOld)
			})

			It("changing the source vxlan vni, should return a new device", func() {
				vxlanOld.Vni = 200
				vxlanDev, err := NewVxlanDevice(vxlanOld)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(vxlanDev).ShouldNot(BeNil())
				checkConfig(vxlanDev, vxlanOld)
			})

			It("changing the source address, should return a new device", func() {
				vxlanOld.VtepPort = 1111
				vxlanDev, err := NewVxlanDevice(vxlanOld)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(vxlanDev).ShouldNot(BeNil())
				checkConfig(vxlanDev, vxlanOld)
			})

			It("existing device is not of type vxlan, should return a new device", func() {
				vxlanOld.Name = "dummy-link"
				vxlanOld.Vni = 123
				vxlanDev, err := NewVxlanDevice(vxlanOld)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(vxlanDev).ShouldNot(BeNil())
				checkConfig(vxlanDev, vxlanOld)
				Expect(deleteLink("vxlan.old")).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("configuring fdb of a vxlan device", func() {
		Context("adding new fdb entry", func() {
			It("fdb does not exist, should return true and nil", func() {
				added, err := vxlanDev.AddFDB(fdbNew)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeTrue())
				fdbs, err := netlink.NeighList(vxlanDev.Link.Index, syscall.AF_BRIDGE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(containsFdbEntry(fdbs, fdbNew)).Should(BeTrue())
			})

			It("fdb does exist, should return nil", func() {
				added, err := vxlanDev.AddFDB(fdbOld)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(added).Should(BeFalse())
				fdbs, err := netlink.NeighList(vxlanDev.Link.Index, syscall.AF_BRIDGE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(containsFdbEntry(fdbs, fdbOld)).Should(BeTrue())
			})
		})

		Context("removing fdb entry", func() {
			It("fdb does not exist, should return nil", func() {
				deleted, err := vxlanDev.DelFDB(fdbNew)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(deleted).Should(BeFalse())
				fdbs, err := netlink.NeighList(vxlanDev.Link.Index, syscall.AF_BRIDGE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(containsFdbEntry(fdbs, fdbNew)).Should(BeFalse())
			})

			It("fdb does exist, should return nil", func() {
				deleted, err := vxlanDev.DelFDB(fdbOld)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(deleted).Should(BeTrue())
				fdbs, err := netlink.NeighList(vxlanDev.Link.Index, syscall.AF_BRIDGE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(containsFdbEntry(fdbs, fdbOld)).Should(BeFalse())
			})
		})
	})

	Describe("configure ip address for vxlan interface", func() {
		Context("ip address is in wrong format", func() {
			It("should return error", func() {
				wrongAddress := "10.234.0.8"
				err := vxlanDev.ConfigureIPAddress(wrongAddress)
				Expect(err).Should(HaveOccurred())
				Expect(err).Should(MatchError(&net.ParseError{
					Type: "CIDR address",
					Text: wrongAddress,
				}))
			})
		})

		Context("ip address is not configured", func() {
			It("should return nil", func() {
				ipAddress := "10.234.0.5/24"
				err := vxlanDev.ConfigureIPAddress(ipAddress)
				Expect(err).ShouldNot(HaveOccurred())
				addresses, err := netlink.AddrList(vxlanDev.Link, netlink.FAMILY_V4)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(addresses)).Should(BeNumerically("==", 1))
				Expect(addresses[0].IPNet.String()).Should(Equal(ipAddress))
			})
		})

		Context("ip address is already configured", func() {
			It("should return nil", func() {
				ipAddress := "10.234.0.5/24"
				ipAddr, err := netlink.ParseIPNet(ipAddress)
				Expect(err).ShouldNot(HaveOccurred())
				err = netlink.AddrAdd(vxlanDev.Link, &netlink.Addr{IPNet: ipAddr})
				Expect(err).ShouldNot(HaveOccurred())
				err = vxlanDev.ConfigureIPAddress(ipAddress)
				Expect(err).ShouldNot(HaveOccurred())
				addresses, err := netlink.AddrList(vxlanDev.Link, netlink.FAMILY_V4)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(addresses)).Should(BeNumerically("==", 1))
				Expect(addresses[0].IPNet.String()).Should(Equal(ipAddress))
			})
		})
	})

	Describe("configure reverse path filtering for vxlan interface", func() {
		Context("setting the rp_filter to 2", func() {
			It("should return nil", func() {
				err := vxlanDev.enableRPFilter()
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})
})
