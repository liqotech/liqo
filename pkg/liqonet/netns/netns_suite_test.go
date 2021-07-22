package netns

import (
	"errors"
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
	// Create a dummy network interface in gateway netns.
	err = netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: existingGatewayVeth}})
	Expect(err).ShouldNot(HaveOccurred())
	err = originNetns.Set()
	// Create dummy network interface in host netns.
	err = netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: existingHostVeth}})
	if !errors.Is(err, unix.EEXIST) {
		Expect(err).ShouldNot(HaveOccurred())
	}
	// Save newly created netns.
	newNetns, err = ns.GetNS("/run/netns/" + netnsName)
	Expect(err).ShouldNot(HaveOccurred())
	defer newNs.Close()
}

func cleanUpEnv() error {
	err := netns.DeleteNamed(netnsName)
	if err != nil && !errors.Is(err, unix.ENOENT) {
		return err
	}
	// Get the veth dev living in host network.
	veth, err := netlink.LinkByName(hostVeth)
	if err != nil && err.Error() != "Link not found" {
		return err
	}
	if veth != nil {
		return netlink.LinkDel(veth)
	}
	return nil
}
