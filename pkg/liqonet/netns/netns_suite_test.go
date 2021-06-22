package netns

import (
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

var (
	netnsName             = "liqo-ns-test"
	originNetns, newNetns ns.NetNS
)

func TestNetns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Netns Suite")
}

var _ = BeforeSuite(func() {
	var err error
	originNetns, err = ns.GetCurrentNS()
	Expect(err).ShouldNot(HaveOccurred())
})

func setUpNetns(name string) {
	var err error
	// Create a new network namespace.
	newNs, err := netns.NewNamed(name)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(newNs).ShouldNot(BeNil())
	// Set the newly created newNs
	err = netns.Set(newNs)
	Expect(err).ShouldNot(HaveOccurred())
	// Create a dummy network interface
	err = netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "foo"}})
	Expect(err).ShouldNot(HaveOccurred())
	err = originNetns.Set()
	Expect(err).ShouldNot(HaveOccurred())
	// Save newly created netns.
	newNetns, err = ns.GetNS("/run/netns/" + netnsName)
	Expect(err).ShouldNot(HaveOccurred())
	defer newNs.Close()
}

func tearDownNetns(name string) {
	err := netns.DeleteNamed(name)
	if err != nil {
		Expect(err).Should(Equal(unix.ENOENT))
		return
	}
	Expect(err).ShouldNot(HaveOccurred())
}
