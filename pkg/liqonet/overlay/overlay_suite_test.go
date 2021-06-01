package overlay

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/liqotech/liqo/pkg/liqonet"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	defaultIfaceIP net.IP
)

func TestOverlay(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Overlay Suite")
}

var _ = BeforeSuite(func() {
	var err error
	defaultIfaceIP, err = getIFaceIP("0.0.0.0")
	Expect(err).ShouldNot(HaveOccurred())
	Expect(defaultIfaceIP).ShouldNot(BeNil())
	// Create dummy link
	err = netlink.LinkAdd(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "dummy-link"}})
	Expect(err).ShouldNot(HaveOccurred())
})

/*var _ = AfterSuite(func() {

})*/

func getIFaceIP(ipAddress string) (net.IP, error) {
	var ifaceIndex int
	// Convert the given IP address from string to net.IP format
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return nil, &liqonet.ParseIPError{
			IPToBeParsed: ipAddress,
		}
	}
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	// Find the route whose destination contains our IP address.
	for i := range routes {
		if routes[i].Scope == netlink.SCOPE_UNIVERSE {
			ifaceIndex = routes[i].LinkIndex
		}
	}
	if ifaceIndex == 0 {
		return nil, &liqonet.NoRouteFound{IPAddress: ipAddress}
	}
	// Get link.
	link, err := netlink.LinkByIndex(ifaceIndex)
	if err != nil {
		return nil, err
	}
	ips, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	return ips[0].IP, nil
}
