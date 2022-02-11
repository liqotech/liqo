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
	"errors"
	"net"
	"os"
	"reflect"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoneterrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

const (
	// DefaultScope is the default value for the route scope.
	DefaultScope netlink.Scope = 0
	// DefaultFlags is the default value for the route flags.
	DefaultFlags int = 0
)

// AddRoute adds a new route on the given interface.
func AddRoute(dstNet, gwIP string, iFaceIndex, tableID, flags int, scope netlink.Scope) (bool, error) {
	var route *netlink.Route
	var gatewayIP net.IP
	// Convert destination in *net.IPNet.
	_, destinationNet, err := net.ParseCIDR(dstNet)
	if err != nil {
		return false, err
	}
	// If gwIP is not set then skip this section.
	if gwIP != "" {
		gatewayIP, err = parseIP(gwIP)
		if err != nil {
			return false, err
		}
	}
	route = &netlink.Route{
		Table:     tableID,
		Dst:       destinationNet,
		Gw:        gatewayIP,
		LinkIndex: iFaceIndex,
		Flags:     flags,
		Scope:     scope,
	}
	// Check if already exists a route for the given destination.
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, route, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_DST)
	if err != nil {
		return false, err
	}
	// For the given destination there should be one and only one route on the given routing table. Keep in mind that the routing
	// table is managed by us. No other processes should mess with it.
	if len(routes) == 1 {
		r := routes[0]
		// Check if the existing rule is equal to the one that we want to configure.
		if reflect.DeepEqual(r.Gw, gatewayIP.To4()) && r.LinkIndex == iFaceIndex {
			klog.V(5).Infof("route {%s} already exists", route.String())
			return false, nil
		}
		// Otherwise remove the outdated route.
		if err := netlink.RouteDel(&r); err != nil {
			return false, err
		}
	}
	klog.V(5).Infof("inserting route {%s}", route.String())
	if err := netlink.RouteAdd(route); err != nil {
		return false, err
	}
	return true, nil
}

// DelRoute removes a route described by the given parameters.
func DelRoute(dstNet, gwIP string, iFaceIndex, tableID int) (bool, error) {
	var route *netlink.Route
	var gatewayIP net.IP
	// Convert destination in *net.IPNet.
	_, destinationNet, err := net.ParseCIDR(dstNet)
	if err != nil {
		return false, err
	}
	// If gwIP is not set then skip this section.
	if gwIP != "" {
		gatewayIP, err = parseIP(gwIP)
		if err != nil {
			return false, err
		}
	}
	route = &netlink.Route{
		Table:     tableID,
		Dst:       destinationNet,
		Gw:        gatewayIP,
		LinkIndex: iFaceIndex,
	}
	// Try to remove all the routes for current dstNet.
	klog.V(5).Infof("deleting route {%s}", route.String())
	err = netlink.RouteDel(route)
	if err != nil {
		if errors.Is(err, unix.ESRCH) {
			// It means the route does not exist so we are done.
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func flushRoutesForRoutingTable(tableID int) error {
	// First we list all the routes contained in the routing table.
	route := &netlink.Route{
		Table: tableID,
	}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, route, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}
	// Delete all the listed rules.
	for i := range routes {
		klog.V(5).Infof("deleting route {%s}", routes[i].String())
		if err := netlink.RouteDel(&routes[i]); err != nil {
			return err
		}
	}
	return nil
}

// AddPolicyRoutingRule adds a new policy routing rule.
func AddPolicyRoutingRule(fromSubnet, toSubnet string, tableID int) (bool, error) {
	var destinationNet, sourceNet *net.IPNet
	var err error

	// Check that at least one between source and destination networks are defined.
	if sourceNet, destinationNet, err = validatePolicyRoutingRulesParameters(fromSubnet, toSubnet); err != nil {
		return false, err
	}
	// Get existing rules.
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return false, err
	}
	// Check if the rule already exists
	for i := range rules {
		if reflect.DeepEqual(destinationNet, rules[i].Dst) && reflect.DeepEqual(sourceNet, rules[i].Src) && rules[i].Table == tableID {
			klog.V(5).Infof("rule %s already exists", rules[i].String())
			return false, nil
		}
	}
	// Remove all the rules which has same destination and source networks.
	for i := range rules {
		if reflect.DeepEqual(destinationNet, rules[i].Dst) && reflect.DeepEqual(sourceNet, rules[i].Src) {
			if err := netlink.RuleDel(&rules[i]); err != nil {
				return false, err
			}
		}
	}
	rule := netlink.NewRule()
	rule.Table = tableID
	rule.Src = sourceNet
	rule.Dst = destinationNet
	klog.V(5).Infof("inserting policy routing rule {%s}", rule.String())
	if err := netlink.RuleAdd(rule); err != nil {
		return false, err
	}
	return true, nil
}

// DelPolicyRoutingRule removes a policy routing rule described by the given parameters.
func DelPolicyRoutingRule(fromSubnet, toSubnet string, tableID int) (bool, error) {
	var destinationNet, sourceNet *net.IPNet
	var err error

	// Check that at least one between source and destination networks are defined.
	if sourceNet, destinationNet, err = validatePolicyRoutingRulesParameters(fromSubnet, toSubnet); err != nil {
		return false, err
	}
	// Get existing rules.
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return false, err
	}
	// Check if the rule already exists.
	for i := range rules {
		if reflect.DeepEqual(destinationNet, rules[i].Dst) && reflect.DeepEqual(sourceNet, rules[i].Src) && rules[i].Table == tableID {
			klog.V(5).Infof("removing policy routing rule {%s}", rules[i].String())
			if err := netlink.RuleDel(&rules[i]); err != nil && !errors.Is(err, unix.ESRCH) {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func flushRulesForRoutingTable(routingTableID int) error {
	// First we list all the policy routing rules.
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		return err
	}
	// Delete all the listed rules that refer to the given routing table.
	for i := range rules {
		// Skip the rules not referring to the given routing table.
		if rules[i].Table != routingTableID {
			continue
		}
		klog.V(5).Infof("deleting rules {%s}", rules[i].String())
		if err := netlink.RuleDel(&rules[i]); err != nil {
			return err
		}
	}
	return nil
}

func getRouteConfig(tep *v1alpha1.TunnelEndpoint, podIP string) (dstPodCIDRNet, dstExternalCIDRNet, gatewayIP string, iFaceIndex int, err error) {
	_, dstPodCIDRNet = utils.GetPodCIDRS(tep)
	_, dstExternalCIDRNet = utils.GetExternalCIDRS(tep)
	// Check if we are running on the same host as the gateway pod.
	if tep.Status.GatewayIP != podIP {
		// If the pod is not running on the same host then set the IP address of the Gateway as next hop.
		gatewayIP = tep.Status.GatewayIP
		// Get the iFace index for the IP address of the Gateway pod.
		iFaceIndex, err = getIFaceIndexForIP(gatewayIP)
		if err != nil {
			return dstPodCIDRNet, dstExternalCIDRNet, gatewayIP, iFaceIndex, err
		}
	} else {
		// Running on the same host as the Gateway then set the index of the veth device living on the same network namespace.
		iFaceIndex = tep.Status.VethIFaceIndex
	}
	return dstPodCIDRNet, dstExternalCIDRNet, gatewayIP, iFaceIndex, err
}

func getIFaceIndexForIP(ipAddress string) (int, error) {
	// Convert the given IP address from string to net.IP format
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return 0, &liqoneterrors.ParseIPError{
			IPToBeParsed: ipAddress,
		}
	}
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return 0, err
	}
	// Find the route whose destination contains our IP address.
	for i := range routes {
		// Skip routes whose Dst field is nil
		if routes[i].Dst == nil {
			continue
		}
		if routes[i].Dst.Contains(ip) {
			return routes[i].LinkIndex, nil
		}
	}
	return 0, &liqoneterrors.NoRouteFound{IPAddress: ipAddress}
}

func validatePolicyRoutingRulesParameters(fromSubnet, toSubnet string) (sourceNet, destinationNet *net.IPNet, err error) {
	// Check that at least one between source and destination networks are defined.
	if fromSubnet == "" && toSubnet == "" {
		return nil, nil, &liqoneterrors.WrongParameter{
			Reason:    liqoneterrors.AtLeastOneValid,
			Parameter: "fromSubnet and toSubnet",
		}
	}
	// If toSubnet is empty string than do not parse it.
	if toSubnet != "" {
		// Convert destination network in *net.IPNet.
		_, destinationNet, err = net.ParseCIDR(toSubnet)
		if err != nil {
			return nil, nil, err
		}
	}
	// If fromSubnet is empty string than do not parse it.
	if fromSubnet != "" {
		// Convert source network in *net.IPNet.
		_, sourceNet, err = net.ParseCIDR(fromSubnet)
		if err != nil {
			return nil, nil, err
		}
	}
	return sourceNet, destinationNet, err
}

func parseIP(ip string) (net.IP, error) {
	address := net.ParseIP(ip)
	if address == nil {
		return address, &liqoneterrors.ParseIPError{
			IPToBeParsed: ip,
		}
	}
	return address, nil
}

// EnableIPForwarding enables ipv4 forwarding in the current network namespace.
func EnableIPForwarding() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0600)
}
