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
	"fmt"
	"net"
	"strconv"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqoerrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

// VxlanRoutingManager implements the routing manager interface.
// Used when nodes are not in the same network.
type VxlanRoutingManager struct {
	routingTableID int
	podIP          string
	vxlanNetPrefix string
	vxlanDevice    *overlay.VxlanDevice
}

// NewVxlanRoutingManager returns a VxlanRoutingManager ready to be used or an error.
func NewVxlanRoutingManager(routingTableID int, podIP, vxlanNetPrefix string, vxlanDevice *overlay.VxlanDevice) (Routing, error) {
	// Check the validity of input parameters.
	if routingTableID > unix.RT_TABLE_MAX {
		return nil, &liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.MinorOrEqual + strconv.Itoa(unix.RT_TABLE_MAX)}
	}
	if routingTableID < 0 {
		return nil, &liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.GreaterOrEqual + strconv.Itoa(0)}
	}
	if vxlanDevice == nil {
		return nil, &liqoerrors.WrongParameter{Parameter: "vxlanDevice", Reason: liqoerrors.NotNil}
	}
	if vxlanNetPrefix == "" {
		return nil, &liqoerrors.WrongParameter{Parameter: "vxlanNetPrefix", Reason: liqoerrors.StringNotEmpty}
	}
	ip := net.ParseIP(podIP)
	if ip == nil {
		return nil, &liqoerrors.ParseIPError{
			IPToBeParsed: podIP,
		}
	}
	klog.Infof("starting Vxlan Routing Manager with routing table ID {%d}, podIP {%s}, vxlanNetPrefix {%s}, vxlanDevice {%s}",
		routingTableID, podIP, vxlanNetPrefix, vxlanDevice.Link.Name)
	vrm := &VxlanRoutingManager{
		routingTableID: routingTableID,
		podIP:          podIP,
		vxlanDevice:    vxlanDevice,
		vxlanNetPrefix: vxlanNetPrefix,
	}
	// Configure IP address of the vxlan interface
	overlayIP := utils.GetOverlayIP(podIP)
	overlayIPCIDR := overlayIP + liqoconst.OverlayNetworkMask
	if err := vrm.vxlanDevice.ConfigureIPAddress(overlayIPCIDR); err != nil {
		return nil, err
	}
	return vrm, nil
}

// EnsureRoutesPerCluster accepts as input a netv1alpha.tunnelendpoint.
// It inserts the routes if they do not exist or updates them if they are outdated.
// Returns true if the routes have been configured, false if the routes are already configured.
// An error if something goes wrong and the routes can not be configured.
func (vrm *VxlanRoutingManager) EnsureRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error) {
	var routePodCIDRAdd, routeExternalCIDRAdd, policyRulePodCIDRAdd, policyRuleExternalCIDRAdd, configured bool
	var iFaceIndex int
	var iFaceName string
	var gatewayIP string
	var err error

	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	_, dstPodCIDR := utils.GetPodCIDRS(tep)
	_, dstExternalCIDR := utils.GetExternalCIDRS(tep)

	if tep.Status.GatewayIP != vrm.podIP {
		gatewayIP = utils.GetOverlayIP(tep.Status.GatewayIP)
		iFaceIndex = vrm.vxlanDevice.Link.Index
		iFaceName = vrm.vxlanDevice.Link.Name

		// If we are not on the same pod as the gateway then make sure that we do not have
		// policy routing rules for the incoming traffic from the remote cluster.
		if config, err := vrm.removePRRForIncomingTraffic(dstPodCIDR, dstExternalCIDR, clusterID); err != nil {
			return config, err
		}
	} else {
		gatewayIP = tep.Status.VethIP
		iFaceIndex = tep.Status.VethIFaceIndex
		iFaceName = tep.Status.VethIFaceName

		if config, err := vrm.ensurePRRForIncomingTraffic(dstPodCIDR, dstExternalCIDR, clusterID); err != nil {
			return config, err
		}
	}

	// Add policy routing rules for the given cluster.
	klog.V(5).Infof("%s -> adding policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstPodCIDR, vrm.routingTableID)
	if policyRulePodCIDRAdd, err = AddPolicyRoutingRule("", dstPodCIDR, vrm.routingTableID); err != nil {
		return policyRulePodCIDRAdd, fmt.Errorf("%s -> unable to add policy routing rule for destination {%s} to lookup routing table with ID {%d}: %w",
			clusterID, dstPodCIDR, vrm.routingTableID, err)
	}
	klog.V(5).Infof("%s -> adding policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstExternalCIDR, vrm.routingTableID)
	if policyRuleExternalCIDRAdd, err = AddPolicyRoutingRule("", dstExternalCIDR, vrm.routingTableID); err != nil {
		return policyRuleExternalCIDRAdd, fmt.Errorf("%s -> unable to add policy routing rule for destination {%s}"+
			" to lookup routing table with ID {%d}: %w",
			clusterID, dstExternalCIDR, vrm.routingTableID, err)
	}

	// Add routes for the given cluster.
	klog.V(5).Infof("%s -> adding route for destination {%s} with gateway {%s} in routing table with ID {%d} on device {%s}",
		clusterID, dstPodCIDR, gatewayIP, vrm.routingTableID, iFaceName)
	routePodCIDRAdd, err = AddRoute(dstPodCIDR, gatewayIP, iFaceIndex, vrm.routingTableID, DefaultFlags, DefaultScope)
	if err != nil {
		return routePodCIDRAdd, fmt.Errorf("%s -> unable to add route for destination {%s} with gateway {%s} "+
			"in routing table with ID {%d} on device {%s}: %w",
			clusterID, dstPodCIDR, gatewayIP, vrm.routingTableID, iFaceName, err)
	}
	klog.V(5).Infof("%s -> adding route for destination {%s} with gateway {%s} in routing table with ID {%d} on device {%s}",
		clusterID, dstExternalCIDR, gatewayIP, vrm.routingTableID, iFaceName)
	routeExternalCIDRAdd, err = AddRoute(dstExternalCIDR, gatewayIP, iFaceIndex, vrm.routingTableID, DefaultFlags, DefaultScope)
	if err != nil {
		return routeExternalCIDRAdd, fmt.Errorf("%s -> unable to add route for destination {%s} with gateway "+
			"{%s} in routing table with ID {%d} on device {%s}: %w",
			clusterID, dstExternalCIDR, gatewayIP, vrm.routingTableID, iFaceName, err)
	}
	if routePodCIDRAdd || routeExternalCIDRAdd || policyRulePodCIDRAdd || policyRuleExternalCIDRAdd {
		configured = true
	}
	return configured, nil
}

// RemoveRoutesPerCluster accepts as input a netv1alpha.tunnelendpoint.
// It deletes the routes if they do exist.
// Returns true if the routes exist and have been deleted, false if nothing is removed.
// An error if something goes wrong and the routes can not be removed.
func (vrm *VxlanRoutingManager) RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error) {
	var routePodCIDRDel, routeExternalCIDRDel, policyRulePodCIDRDel, policyRuleExternalCIDRDel, configured bool
	var iFaceIndex int
	var iFaceName string
	var gatewayIP string
	var err error

	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	_, dstPodCIDR := utils.GetPodCIDRS(tep)
	_, dstExternalCIDR := utils.GetExternalCIDRS(tep)

	if tep.Status.GatewayIP != vrm.podIP {
		gatewayIP = utils.GetOverlayIP(tep.Status.GatewayIP)
		iFaceIndex = vrm.vxlanDevice.Link.Index
		iFaceName = vrm.vxlanDevice.Link.Name
	} else {
		gatewayIP = tep.Status.VethIP
		iFaceIndex = tep.Status.VethIFaceIndex
		iFaceName = tep.Status.VethIFaceName

		// If we are on the same node as the gateway than remove also the policy routing rules
		// for the incoming traffic from the remote cluster.
		if config, err := vrm.removePRRForIncomingTraffic(dstPodCIDR, dstExternalCIDR, clusterID); err != nil {
			return config, err
		}
	}

	// Delete policy routing rules for the given cluster.
	klog.V(5).Infof("%s -> deleting policy routing rule for destination {%s} with table ID {%d}",
		clusterID, dstPodCIDR, vrm.routingTableID)
	if policyRulePodCIDRDel, err = DelPolicyRoutingRule("", dstPodCIDR, vrm.routingTableID); err != nil {
		return policyRulePodCIDRDel, fmt.Errorf("%s -> unable to delete policy routing rule for destination {%s} with table ID {%d}: %w",
			clusterID, dstPodCIDR, vrm.routingTableID, err)
	}
	klog.V(5).Infof("%s -> deleting policy routing rule for destination {%s} with table with ID {%d}",
		clusterID, dstExternalCIDR, vrm.routingTableID)
	if policyRuleExternalCIDRDel, err = DelPolicyRoutingRule("", dstExternalCIDR, vrm.routingTableID); err != nil {
		return policyRuleExternalCIDRDel, fmt.Errorf("%s -> unable to delete policy routing rule for destination {%s} with table ID {%d}: %w",
			clusterID, dstExternalCIDR, vrm.routingTableID, err)
	}

	// Delete routes for the given cluster.
	klog.V(5).Infof("%s -> deleting route for destination {%s} with gateway {%s} in "+
		"routing table with ID {%d} on device {%s}",
		clusterID, dstPodCIDR, gatewayIP, vrm.routingTableID, iFaceName)
	routePodCIDRDel, err = DelRoute(dstPodCIDR, gatewayIP, iFaceIndex, vrm.routingTableID)
	if err != nil {
		return routePodCIDRDel, fmt.Errorf("%s -> unable to delete route for destination {%s} with gateway {%s} "+
			"in routing table with ID {%d} on device {%s}: %w",
			clusterID, dstPodCIDR, gatewayIP, vrm.routingTableID, iFaceName, err)
	}
	klog.V(5).Infof("%s -> deleting route for destination {%s} with gateway {%s} "+
		"in routing table with ID {%d} on device {%s}",
		clusterID, dstExternalCIDR, gatewayIP, vrm.routingTableID, iFaceName)
	routeExternalCIDRDel, err = DelRoute(dstExternalCIDR, gatewayIP, iFaceIndex, vrm.routingTableID)
	if err != nil {
		return routeExternalCIDRDel, fmt.Errorf("%s -> unable to delete route for destination {%s} with gateway {%s} "+
			"in routing table with ID {%d} on device {%s}: %w",
			clusterID, dstExternalCIDR, gatewayIP, vrm.routingTableID, iFaceName, err)
	}

	if policyRulePodCIDRDel || policyRuleExternalCIDRDel || routePodCIDRDel || routeExternalCIDRDel {
		configured = true
	}
	return configured, nil
}

// CleanRoutingTable removes all the routes from the custom routing table used by the route manager.
func (vrm *VxlanRoutingManager) CleanRoutingTable() error {
	klog.Infof("flushing routing table with ID {%d}", vrm.routingTableID)
	return flushRoutesForRoutingTable(vrm.routingTableID)
}

// CleanPolicyRules removes all the policy rules pointing to the custom routing table used by the route manager.
func (vrm *VxlanRoutingManager) CleanPolicyRules() error {
	klog.Infof("removing all policy routing rules that reference routing table with ID {%d}", vrm.routingTableID)
	return flushRulesForRoutingTable(vrm.routingTableID)
}

func (vrm *VxlanRoutingManager) ensurePRRForIncomingTraffic(srcPodCIDR, srcExternalCIDR, clusterID string) (bool, error) {
	var configured bool

	klog.V(5).Infof("%s -> adding policy routing rule for source podCIDR {%s} to lookup routing table with ID {%d}",
		clusterID, srcPodCIDR, vrm.routingTableID)
	policyRulePodCIDRAdd, err := AddPolicyRoutingRule(srcPodCIDR, "", liqoconst.RoutingTableID)
	if err != nil {
		return false, fmt.Errorf("%s -> unable to add policy routing rule for source podCIDR {%s} to lookup routing table with ID {%d}: %w",
			clusterID, srcPodCIDR, vrm.routingTableID, err)
	}

	klog.V(5).Infof("%s -> adding policy routing rule for source externalCIDR {%s} to lookup routing table with ID {%d}",
		clusterID, srcPodCIDR, vrm.routingTableID)
	policyRuleExternalCIDRAdd, err := AddPolicyRoutingRule(srcExternalCIDR, "", liqoconst.RoutingTableID)
	if err != nil {
		return false, fmt.Errorf("%s -> unable to add policy routing rule for source externalCIDR {%s} to lookup routing table with ID {%d}: %w",
			clusterID, srcPodCIDR, vrm.routingTableID, err)
	}

	if policyRulePodCIDRAdd || policyRuleExternalCIDRAdd {
		configured = true
	}

	return configured, nil
}

func (vrm *VxlanRoutingManager) removePRRForIncomingTraffic(srcPodCIDR, srcExternalCIDR, clusterID string) (bool, error) {
	var configured bool

	klog.V(5).Infof("%s -> deleting policy routing rule for source podCIDR {%s} to lookup routing table with ID {%d}",
		clusterID, srcPodCIDR, vrm.routingTableID)
	policyRulePodCIDRDel, err := DelPolicyRoutingRule(srcPodCIDR, "", liqoconst.RoutingTableID)
	if err != nil {
		return false, fmt.Errorf("%s -> unable to delete policy routing rule for source podCIDR {%s} to lookup routing table with ID {%d}: %w",
			clusterID, srcPodCIDR, vrm.routingTableID, err)
	}

	klog.V(5).Infof("%s -> deleting policy routing rule for source externalCIDR {%s} to lookup routing table with ID {%d}",
		clusterID, srcPodCIDR, vrm.routingTableID)
	policyRuleExternalCIDRDel, err := DelPolicyRoutingRule(srcExternalCIDR, "", liqoconst.RoutingTableID)
	if err != nil {
		return false, fmt.Errorf("%s -> unable to delete policy routing rule for source externalCIDR {%s} to lookup routing table with ID {%d}: %w",
			clusterID, srcPodCIDR, vrm.routingTableID, err)
	}

	if policyRulePodCIDRDel || policyRuleExternalCIDRDel {
		configured = true
	}

	return configured, nil
}
