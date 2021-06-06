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

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
)

var (
	ipAddress1         = "10.0.0.1/24"
	ipAddress1NoSubnet = "10.0.0.1"
	ipAddress2         = "10.0.0.2/24"
	ipAddress2NoSubnet = "10.0.0.2"
	// The value of ipAddress2NoSubnet when is mapped to the overlay network.
	ipAddress2NoSubnetOverlay = "240.0.0.2"
	dummylink1, dummyLink2    netlink.Link
	iFacesNames               = []string{"liqo-test-1", "liqo-test-2"}
	drm, vrm                  Routing

	tep netv1alpha1.TunnelEndpoint
)

type routingInfo struct {
	destinationNet string
	gatewayIP      string
	iFaceIndex     int
	routingTableID int
}

func TestRouting(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Routing Suite")
}

var _ = BeforeSuite(func() {
	var err error
	setUpInterfaces()
	drm, err = NewDirectRoutingManager(routingTableIDDRM, ipAddress1NoSubnet)
	Expect(err).Should(BeNil())
	Expect(drm).NotTo(BeNil())
	tep = netv1alpha1.TunnelEndpoint{
		Status: netv1alpha1.TunnelEndpointStatus{
			LocalNATPodCIDR:  "10.150.0.0/16",
			RemoteNATPodCIDR: "10.250.0.0/16",
			VethIFaceIndex:   12345,
			GatewayIP:        ipAddress2NoSubnet,
		}}

	// Create vxlan device for Vxlan Routing manager tests.
	link, err := setUpVxlanLink(vxlanConfig)
	Expect(err).ShouldNot(HaveOccurred())
	overlayDevice = overlay.VxlanDevice{Link: link.(*netlink.Vxlan)}
	// Create Vxlan Routing Manager.
	vrm, err = NewVxlanRoutingManager(routingTableIDVRM, ipAddress1NoSubnet, overlayNetPrexif, overlayDevice)
	Expect(err).Should(BeNil())
	Expect(vrm).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	tearDownInterfaces()
	deleteLink(vxlanConfig.Name)
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

func setUpRoutes(routes []routingInfo) []*netlink.Route {
	r := make([]*netlink.Route, 2)
	for i := range routes {
		var gwIP net.IP
		var route *netlink.Route
		// Parse destination network.
		_, dstNet, err := net.ParseCIDR(routes[i].destinationNet)
		Expect(err).Should(BeNil())
		// Parse gateway ip address if set.
		if routes[i].gatewayIP != "" {
			gwIP = net.ParseIP(routes[i].gatewayIP)
			Expect(gwIP).ShouldNot(BeNil())
		}
		route = &netlink.Route{Dst: dstNet, Gw: gwIP, Table: routes[i].routingTableID, LinkIndex: routes[i].iFaceIndex}
		err = netlink.RouteAdd(route)
		Expect(err).Should(BeNil())
		r[i] = route
		rule := netlink.NewRule()
		rule.Dst = dstNet
		rule.Table = routes[i].routingTableID
		Expect(netlink.RuleAdd(rule)).Should(BeNil())
	}
	return r
}

func tearDownRoutes(tableID int) {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Table: tableID}, netlink.RT_FILTER_TABLE)
	Expect(err).Should(BeNil())
	for i := range routes {
		if routes[i].Table == tableID {
			Expect(netlink.RouteDel(&routes[i])).Should(BeNil())
		}
	}
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	Expect(err).Should(BeNil())
	for i := range rules {
		if rules[i].Table == tableID || rules[i].Table == 12345 {
			Expect(netlink.RuleDel(&rules[i])).Should(BeNil())
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
