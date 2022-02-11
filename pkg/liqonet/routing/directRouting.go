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
	"net"
	"strconv"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet/errors"
)

// DirectRoutingManager implements the routing manager interface.
// Nodes has to be on the same network otherwise this solution will not work.
type DirectRoutingManager struct {
	routingTableID int
	podIP          string
}

// NewDirectRoutingManager accepts as input a routing table ID and the IP address of the pod.
// Returns a DirectRoutingManager ready to be used or an error.
func NewDirectRoutingManager(routingTableID int, podIP string) (Routing, error) {
	klog.Infof("starting Direct Routing Manager with routing table ID %d and podIP %s", routingTableID, podIP)
	// Check the validity of input parameters.
	if routingTableID > unix.RT_TABLE_MAX {
		return nil, &errors.WrongParameter{Parameter: "routingTableID", Reason: errors.MinorOrEqual + strconv.Itoa(unix.RT_TABLE_MAX)}
	}
	if routingTableID < 0 {
		return nil, &errors.WrongParameter{Parameter: "routingTableID", Reason: errors.GreaterOrEqual + strconv.Itoa(0)}
	}
	ip := net.ParseIP(podIP)
	if ip == nil {
		return nil, &errors.ParseIPError{
			IPToBeParsed: podIP,
		}
	}
	return &DirectRoutingManager{
		routingTableID: routingTableID,
		podIP:          podIP,
	}, nil
}

// EnsureRoutesPerCluster accepts as input a netv1alpha.tunnelendpoint.
// It inserts the routes if they do not exist or updates them if they are outdated.
// Returns true if the routes have been configured, false if the routes are already configured.
// An error if something goes wrong and the routes can not be configured.
func (drm *DirectRoutingManager) EnsureRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error) {
	var routePodCIDRAdd, routeExternalCIDRAdd, policyRulePodCIDRAdd, policyRuleExternalCIDRAdd, configured bool
	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	dstPodCIDR, dstExternalCIDR, gatewayIP, iFaceIndex, err := getRouteConfig(tep, drm.podIP)
	if err != nil {
		return false, err
	}
	// Add policy routing rules for the given cluster.
	klog.Infof("%s -> adding policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstPodCIDR, drm.routingTableID)
	if policyRulePodCIDRAdd, err = AddPolicyRoutingRule("", dstPodCIDR, drm.routingTableID); err != nil {
		return policyRulePodCIDRAdd, err
	}
	klog.Infof("%s -> adding policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstExternalCIDR, drm.routingTableID)
	if policyRuleExternalCIDRAdd, err = AddPolicyRoutingRule("", dstExternalCIDR, drm.routingTableID); err != nil {
		return policyRuleExternalCIDRAdd, err
	}
	// Add routes for the given cluster.
	klog.Infof("%s -> adding route for destination {%s} with gateway {%s} in routing table with ID {%d}",
		clusterID, dstPodCIDR, gatewayIP, drm.routingTableID)
	routePodCIDRAdd, err = AddRoute(dstPodCIDR, gatewayIP, iFaceIndex, drm.routingTableID, DefaultFlags, DefaultScope)
	if err != nil {
		return routePodCIDRAdd, err
	}
	klog.Infof("%s -> adding route for destination {%s} with gateway {%s} in routing table with ID {%d}",
		clusterID, dstExternalCIDR, gatewayIP, drm.routingTableID)
	routeExternalCIDRAdd, err = AddRoute(dstExternalCIDR, gatewayIP, iFaceIndex, drm.routingTableID, DefaultFlags, DefaultScope)
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
func (drm *DirectRoutingManager) RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error) {
	var routePodCIDRDel, routeExternalCIDRDel, policyRulePodCIDRDel, policyRuleExternalCIDRDel, configured bool
	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	dstPodCIDR, dstExternalCIDR, gatewayIP, iFaceIndex, err := getRouteConfig(tep, drm.podIP)
	if err != nil {
		return false, err
	}
	// Delete policy routing rules for the given cluster.
	klog.Infof("%s -> deleting policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstPodCIDR, drm.routingTableID)
	if policyRulePodCIDRDel, err = DelPolicyRoutingRule("", dstPodCIDR, drm.routingTableID); err != nil {
		return policyRulePodCIDRDel, err
	}
	klog.Infof("%s -> deleting policy routing rule for destination {%s} to lookup routing table with ID {%d}",
		clusterID, dstExternalCIDR, drm.routingTableID)
	if policyRuleExternalCIDRDel, err = DelPolicyRoutingRule("", dstExternalCIDR, drm.routingTableID); err != nil {
		return policyRuleExternalCIDRDel, err
	}
	// Delete routes for the given cluster.
	klog.Infof("%s -> deleting route for destination {%s} with gateway {%s} in routing table with ID {%d}",
		clusterID, dstPodCIDR, gatewayIP, drm.routingTableID)
	routePodCIDRDel, err = DelRoute(dstPodCIDR, gatewayIP, iFaceIndex, drm.routingTableID)
	if err != nil {
		return routePodCIDRDel, err
	}
	klog.Infof("%s -> deleting route for destination {%s} with gateway {%s} in routing table with ID {%d}",
		clusterID, dstExternalCIDR, gatewayIP, drm.routingTableID)
	routeExternalCIDRDel, err = DelRoute(dstExternalCIDR, gatewayIP, iFaceIndex, drm.routingTableID)
	if err != nil {
		return routeExternalCIDRDel, err
	}
	if routePodCIDRDel || routeExternalCIDRDel || policyRulePodCIDRDel || policyRuleExternalCIDRDel {
		configured = true
	}
	return configured, nil
}

// CleanRoutingTable removes all the routes from the custom routing table used by the route manager.
func (drm *DirectRoutingManager) CleanRoutingTable() error {
	return flushRoutesForRoutingTable(drm.routingTableID)
}

// CleanPolicyRules removes all the policy rules pointing to the custom routing table used by the route manager.
func (drm *DirectRoutingManager) CleanPolicyRules() error {
	return flushRulesForRoutingTable(drm.routingTableID)
}
