// Copyright 2019-2021 The Liqo Authors
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
	var gatewayIP string
	var err error
	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	_, dstPodCIDR := utils.GetPodCIDRS(tep)
	_, dstExternalCIDR := utils.GetExternalCIDRS(tep)
	if tep.Status.GatewayIP != vrm.podIP {
		gatewayIP = utils.GetOverlayIP(tep.Status.GatewayIP)
		iFaceIndex = vrm.vxlanDevice.Link.Index
	} else {
		iFaceIndex = tep.Status.VethIFaceIndex
	}
	// Add policy routing rule for the given cluster.
	klog.V(4).Infof("%s -> adding policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstPodCIDR, vrm.routingTableID)
	if policyRulePodCIDRAdd, err = AddPolicyRoutingRule("", dstPodCIDR, vrm.routingTableID); err != nil {
		return policyRulePodCIDRAdd, err
	}
	klog.V(4).Infof("%s -> adding policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstExternalCIDR, vrm.routingTableID)
	if policyRuleExternalCIDRAdd, err = AddPolicyRoutingRule("", dstExternalCIDR, vrm.routingTableID); err != nil {
		return policyRuleExternalCIDRAdd, err
	}
	// Add route for the given cluster.
	klog.V(4).Infof("%s -> adding route for destination {%s} with gateway {%s} in routing table with ID {%d} on device {%s}",
		clusterID, dstPodCIDR, gatewayIP, vrm.routingTableID, vrm.vxlanDevice.Link.Name)
	routePodCIDRAdd, err = AddRoute(dstPodCIDR, gatewayIP, iFaceIndex, vrm.routingTableID)
	if err != nil {
		return routePodCIDRAdd, err
	}
	klog.V(4).Infof("%s -> adding route for destination {%s} with gateway {%s} in routing table with ID {%d} on device {%s}",
		clusterID, dstExternalCIDR, gatewayIP, vrm.routingTableID, vrm.vxlanDevice.Link.Name)
	routeExternalCIDRAdd, err = AddRoute(dstExternalCIDR, gatewayIP, iFaceIndex, vrm.routingTableID)
	if err != nil {
		return routeExternalCIDRAdd, err
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
	var gatewayIP string
	var err error
	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	_, dstPodCIDRNet := utils.GetPodCIDRS(tep)
	_, dstExternalCIDRNet := utils.GetExternalCIDRS(tep)
	if tep.Status.GatewayIP != vrm.podIP {
		gatewayIP = utils.GetOverlayIP(tep.Status.GatewayIP)
		iFaceIndex = vrm.vxlanDevice.Link.Index
	} else {
		iFaceIndex = tep.Status.VethIFaceIndex
	}
	// Delete policy routing rule for the given cluster.
	klog.V(4).Infof("%s -> deleting policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstPodCIDRNet, vrm.routingTableID)
	if policyRulePodCIDRDel, err = DelPolicyRoutingRule("", dstPodCIDRNet, vrm.routingTableID); err != nil {
		return policyRulePodCIDRDel, err
	}
	klog.V(4).Infof("%s -> deleting policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstExternalCIDRNet, vrm.routingTableID)
	if policyRuleExternalCIDRDel, err = DelPolicyRoutingRule("", dstExternalCIDRNet, vrm.routingTableID); err != nil {
		return policyRuleExternalCIDRDel, err
	}
	// Delete route for the given cluster.
	klog.V(4).Infof("%s -> deleting route for destination {%s} with gateway {%s} in routing table with ID {%d} on device {%s}",
		clusterID, dstPodCIDRNet, gatewayIP, vrm.routingTableID, vrm.vxlanDevice.Link.Name)
	routePodCIDRDel, err = DelRoute(dstPodCIDRNet, gatewayIP, iFaceIndex, vrm.routingTableID)
	if err != nil {
		return routePodCIDRDel, err
	}
	klog.V(4).Infof("%s -> deleting route for destination {%s} with gateway {%s} in routing table with ID {%d} on device {%s}",
		clusterID, dstExternalCIDRNet, gatewayIP, vrm.routingTableID, vrm.vxlanDevice.Link.Name)
	routeExternalCIDRDel, err = DelRoute(dstExternalCIDRNet, gatewayIP, iFaceIndex, vrm.routingTableID)
	if err != nil {
		return routeExternalCIDRDel, err
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
