package routing

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
)

var (
	ipAddress1             = "10.0.0.1/24"
	ipAddress2             = "10.0.0.2/24"
	dummylink1, dummyLink2 netlink.Link
	iFacesNames            = []string{"lioo-test-1", "liqo-test-2"}
)

func TestRouting(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Routing Suite")
}

var _ = BeforeSuite(func() {
	setUpInterfaces()
})

var _ = AfterSuite(func() {
	tearDownInterfaces()
})

func setUpInterfaces() {
	var stdout, stderr bytes.Buffer
	var err error
	// First we create a dummy interfaces used to run the tests.
	for _, iFace := range iFacesNames {
		klog.Infof("creating dummy interface named {%s}", iFace)
		cmd := exec.Command("ip", "link", "add", iFace, "type", "dummy")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			outStr, errStr := stdout.String(), stderr.String()
			fmt.Printf("out:\n%s\nerr:\n%s\n", outStr, errStr)
			klog.Errorf("failed to add dummy interface {liqo-test}: %v", err)
		}
		Expect(err).NotTo(HaveOccurred())
	}

	// Get the dummy interface.
	dummylink1, err = netlink.LinkByName(iFacesNames[0])
	Expect(err).NotTo(HaveOccurred())
	// Set the dummy interface up.
	Expect(netlink.LinkSetUp(dummylink1)).NotTo(HaveOccurred())
	// Assign IP address to dummy interface
	ip, network, err := net.ParseCIDR(ipAddress1)
	Expect(err).NotTo(HaveOccurred())
	Expect(netlink.AddrAdd(dummylink1, &netlink.Addr{IPNet: &net.IPNet{
		IP:   ip,
		Mask: network.Mask,
	}})).NotTo(HaveOccurred())
	// Get the dummy interface.
	dummyLink2, err = netlink.LinkByName(iFacesNames[1])
	Expect(err).NotTo(HaveOccurred())
	// Set the dummy interface up.
	Expect(netlink.LinkSetUp(dummyLink2)).NotTo(HaveOccurred())
	// Assign IP address to dummy interface
	ip, network, err = net.ParseCIDR(ipAddress2)
	Expect(err).NotTo(HaveOccurred())
	Expect(netlink.AddrAdd(dummyLink2, &netlink.Addr{IPNet: &net.IPNet{
		IP:   ip,
		Mask: network.Mask,
	}})).NotTo(HaveOccurred())
}

func tearDownInterfaces() {
	// Remove dummy interfaces.
	for _, iFace := range iFacesNames {
		dummyLink, err := netlink.LinkByName(iFace)
		Expect(err).NotTo(HaveOccurred())
		Expect(netlink.LinkDel(dummyLink)).NotTo(HaveOccurred())
	}
}

func setUpRoutes() {
	dst1 := "10.10.0.0/16"
	gw1 := "10.0.0.10"
	dst2 := "10.11.0.0/24"
	// Add route 1.
	_, dstNet1, err := net.ParseCIDR(dst1)
	Expect(err).Should(BeNil())
	gw := net.ParseIP(gw1)
	Expect(gw).ShouldNot(BeNil())
	err = netlink.RouteAdd(&netlink.Route{Dst: dstNet1, Gw: gw, Table: routingTableID, LinkIndex: dummylink1.Attrs().Index})
	Expect(err).Should(BeNil())
	existingRouteGW = &netlink.Route{Dst: dstNet1, Table: routingTableID, Gw: gw}
	// Add route 2.
	_, dstNet2, err := net.ParseCIDR(dst2)
	Expect(err).Should(BeNil())
	err = netlink.RouteAdd(&netlink.Route{Dst: dstNet2, Table: routingTableID, LinkIndex: dummylink1.Attrs().Index})
	Expect(err).Should(BeNil())
	existingRoute = &netlink.Route{Dst: dstNet2, Table: routingTableID}
}

func tearDownRoutes() {
	// Remove all routes on the custom routing table
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, existingRouteGW, netlink.RT_FILTER_TABLE)
	Expect(err).Should(BeNil())
	for i := range routes {
		if routes[i].Table == routingTableID {
			Expect(netlink.RouteDel(&routes[i])).Should(BeNil())
		}
	}
}

func getRule(fromSubnet, toSubnet string, tableID int) (*netlink.Rule, error) {
	var destinationNet, sourceNet *net.IPNet
	var err error
	if toSubnet != "" {
		// Convert destination network in *net.IPNet.
		_, destinationNet, err = net.ParseCIDR(toSubnet)
		if err != nil {
			return nil, err
		}
	}
	if fromSubnet != "" {
		// Convert source network in *net.IPNet.
		_, sourceNet, err = net.ParseCIDR(fromSubnet)
		if err != nil {
			return nil, err
		}
	}
	// Get existing rules.
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return nil, err
	}
	for i := range rules {
		if reflect.DeepEqual(destinationNet, rules[i].Dst) && reflect.DeepEqual(sourceNet, rules[i].Src) && rules[i].Table == tableID {
			return &rules[i], nil
		}
	}
	return nil, fmt.Errorf("rule not found")
}

func existsRuleForRoutingTable(tableID int) (bool, error) {
	// Get existing rules.
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return false, err
	}
	for i := range rules {
		if rules[i].Table == tableID {
			return true, nil
		}
	}
	return false, nil
}

func setUpRules() {
	dst := "10.10.0.0/16"
	src := "10.11.0.0/24"
	// Add route 1.
	_, dstNet, err := net.ParseCIDR(dst)
	Expect(err).Should(BeNil())
	_, srcNet, err := net.ParseCIDR(src)
	Expect(err).Should(BeNil())
	// Add rule from.
	existingRuleFrom = netlink.NewRule()
	existingRuleFrom.Table = routingTableID
	existingRuleFrom.Src = srcNet
	err = netlink.RuleAdd(existingRuleFrom)
	Expect(err).Should(BeNil())
	// Add rule to.
	existingRuleTo = netlink.NewRule()
	existingRuleTo.Table = routingTableID
	existingRuleTo.Dst = dstNet
	err = netlink.RuleAdd(existingRuleTo)
	Expect(err).Should(BeNil())
}

func tearDownRules() {
	// Get existing rules.
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	Expect(err).Should(BeNil())
	for i := range rules {
		if rules[i].Table == routingTableID || rules[i].Table == 12345 {
			Expect(netlink.RuleDel(&rules[i])).Should(BeNil())
		}
	}
}
