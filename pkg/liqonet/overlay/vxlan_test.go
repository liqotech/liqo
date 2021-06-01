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
	fdbOld, fdbNew        Neighbor
)

func createLink(attrs *VxlanDeviceAttrs) (netlink.Link, error) {
	err := netlink.LinkAdd(&netlink.Vxlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:  vxlanOld.Name,
			MTU:   vxlanOld.Mtu,
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
	Expect(vxlanDev.Link.MTU).Should(BeNumerically("==", vxlanOld.Mtu))
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
			Mtu:      1450,
		}
		vxlanOld = &VxlanDeviceAttrs{
			Vni:      18953,
			Name:     "vxlan.old",
			VtepAddr: defaultIfaceIP,
			VtepPort: 4789,
			Mtu:      1450,
		}
		vxlanOldLink, err = createLink(vxlanOld)
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
			It("fdb does not exist, should return nil", func() {
				vxlanDev := VxlanDevice{Link: vxlanOldLink.(*netlink.Vxlan)}
				Expect(vxlanDev.AddFDB(fdbNew)).ShouldNot(HaveOccurred())
				fdbs, err := netlink.NeighList(vxlanDev.Link.Index, syscall.AF_BRIDGE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(containsFdbEntry(fdbs, fdbNew)).Should(BeTrue())
			})

			It("fdb does exist, should return nil", func() {
				vxlanDev := VxlanDevice{Link: vxlanOldLink.(*netlink.Vxlan)}
				Expect(vxlanDev.AddFDB(fdbOld)).ShouldNot(HaveOccurred())
				fdbs, err := netlink.NeighList(vxlanDev.Link.Index, syscall.AF_BRIDGE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(containsFdbEntry(fdbs, fdbOld)).Should(BeTrue())
			})
		})

		Context("removing fdb entry", func() {
			It("fdb does not exist, should return nil", func() {
				vxlanDev := VxlanDevice{Link: vxlanOldLink.(*netlink.Vxlan)}
				Expect(vxlanDev.DelFDB(fdbNew)).ShouldNot(HaveOccurred())
				fdbs, err := netlink.NeighList(vxlanDev.Link.Index, syscall.AF_BRIDGE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(containsFdbEntry(fdbs, fdbNew)).Should(BeFalse())
			})

			It("fdb does exist, should return nil", func() {
				vxlanDev := VxlanDevice{Link: vxlanOldLink.(*netlink.Vxlan)}
				Expect(vxlanDev.DelFDB(fdbOld)).ShouldNot(HaveOccurred())
				fdbs, err := netlink.NeighList(vxlanDev.Link.Index, syscall.AF_BRIDGE)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(containsFdbEntry(fdbs, fdbOld)).Should(BeFalse())
			})
		})
	})

})
