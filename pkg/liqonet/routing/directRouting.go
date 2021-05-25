package routing

import (
	"net"
	"strconv"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqonet"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
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
		return nil, &liqonet.WrongParameter{Parameter: "routingTableID", Reason: liqonet.MinorOrEqual + strconv.Itoa(unix.RT_TABLE_MAX)}
	}
	if routingTableID < 0 {
		return nil, &liqonet.WrongParameter{Parameter: "routingTableID", Reason: liqonet.GreaterOrEqual + strconv.Itoa(0)}
	}
	ip := net.ParseIP(podIP)
	if ip == nil {
		return nil, &liqonet.ParseIPError{
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
	var routeAdd, policyRuleAdd, configured bool
	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	dstNet, gatewayIP, iFaceIndex, err := getRouteConfig(tep, drm.podIP)
	if err != nil {
		return false, err
	}
	// Add policy routing rule for the given cluster.
	klog.Infof("%s -> adding policy routing rule for destination {%s} to lookup routing table with ID {%d}", clusterID, dstNet, drm.routingTableID)
	if policyRuleAdd, err = addPolicyRoutingRule("", dstNet, drm.routingTableID); err != nil {
		return policyRuleAdd, err
	}
	// Add route for the given cluster.
	klog.Infof("%s -> adding route for destination {%s} with gateway {%s} in routing table with ID {%d}",
		clusterID, dstNet, gatewayIP, drm.routingTableID)
	routeAdd, err = addRoute(dstNet, gatewayIP, iFaceIndex, drm.routingTableID)
	if err != nil {
		return routeAdd, err
	}
	if routeAdd || policyRuleAdd {
		configured = true
	}
	return configured, nil
}

// RemoveRoutesPerCluster accepts as input a netv1alpha.tunnelendpoint.
// It deletes the routes if they do exist.
// Returns true if the routes exist and have been deleted, false if nothing is removed.
// An error if something goes wrong and the routes can not be removed.
func (drm *DirectRoutingManager) RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error) {
	var routeDel, policyRuleDel, configured bool
	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	dstNet, gatewayIP, iFaceIndex, err := getRouteConfig(tep, drm.podIP)
	if err != nil {
		return false, err
	}
	// Delete policy routing rule for the given cluster.
	klog.Infof("%s -> deleting policy routing rule for destination {%s} to lookup routing table with ID {%d}", clusterID, dstNet, drm.routingTableID)
	if policyRuleDel, err = delPolicyRoutingRule("", dstNet, drm.routingTableID); err != nil {
		return policyRuleDel, err
	}
	// Delete route for the given cluster.
	klog.Infof("%s -> deleting route for destination {%s} with gateway {%s} in routing table with ID {%d}",
		clusterID, dstNet, gatewayIP, drm.routingTableID)
	routeDel, err = delRoute(dstNet, gatewayIP, iFaceIndex, drm.routingTableID)
	if err != nil {
		return routeDel, err
	}
	if routeDel || policyRuleDel {
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
