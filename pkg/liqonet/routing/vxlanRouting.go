package routing

import (
	"net"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoerrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/overlay"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

const (
	overlayNetworkPrefix = "240"
	overlayNetworkMask   = "/8"
)

// VxlanRoutingManager implements the routing manager interface.
// Used when nodes are not in the same network.
type VxlanRoutingManager struct {
	routingTableID int
	podIP          string
	vxlanNetPrefix string
	vxlanDevice    overlay.VxlanDevice
}

// NewVxlanRoutingManager returns a VxlanRoutingManager ready to be used or an error.
func NewVxlanRoutingManager(routingTableID int, podIP, vxlanNetPrefix string, vxlanDevice overlay.VxlanDevice) (Routing, error) {
	// Check the validity of input parameters.
	if routingTableID > unix.RT_TABLE_MAX {
		return nil, &liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.MinorOrEqual + strconv.Itoa(unix.RT_TABLE_MAX)}
	}
	if routingTableID < 0 {
		return nil, &liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.GreaterOrEqual + strconv.Itoa(0)}
	}
	if vxlanDevice.Link == nil {
		return nil, &liqoerrors.WrongParameter{Parameter: "vxlanDevice.Link", Reason: liqoerrors.NotNil}
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
	klog.Infof("starting Vxlan Routing Manager with routing table ID %d, podIP %s, vxlanNetPrefix %s, vxlanDevice %s",
		routingTableID, podIP, vxlanNetPrefix, vxlanDevice.Link.Name)
	vrm := &VxlanRoutingManager{
		routingTableID: routingTableID,
		podIP:          podIP,
		vxlanDevice:    vxlanDevice,
		vxlanNetPrefix: vxlanNetPrefix,
	}
	// Configure IP address of the vxlan interface
	overlayIP := vrm.getOverlayIP(podIP)
	overlayIPCIDR := overlayIP + overlayNetworkMask
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
	var routeAdd, policyRuleAdd, configured bool
	var iFaceIndex int
	var gatewayIP string
	var err error
	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	_, dstNet := utils.GetPodCIDRS(tep)
	if tep.Status.GatewayIP != vrm.podIP {
		gatewayIP = vrm.getOverlayIP(tep.Status.GatewayIP)
		iFaceIndex = vrm.vxlanDevice.Link.Index
	} else {
		iFaceIndex = tep.Status.VethIFaceIndex
	}
	// Add policy routing rule for the given cluster.
	klog.Infof("%s -> adding policy routing rule for destination {%s} to lookup routing table with ID {%d}", clusterID, dstNet, vrm.routingTableID)
	if policyRuleAdd, err = addPolicyRoutingRule("", dstNet, vrm.routingTableID); err != nil {
		return policyRuleAdd, err
	}
	// Add route for the given cluster.
	klog.Infof("%s -> adding route for destination {%s} with gateway {%s} in routing table with ID {%d}",
		clusterID, dstNet, gatewayIP, vrm.routingTableID)
	routeAdd, err = AddRoute(dstNet, gatewayIP, iFaceIndex, vrm.routingTableID)
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
func (vrm *VxlanRoutingManager) RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error) {
	var routeDel, policyRuleDel, configured bool
	var iFaceIndex int
	var gatewayIP string
	var err error
	clusterID := tep.Spec.ClusterID
	// Extract and save route information from the given tep.
	_, dstNet := utils.GetPodCIDRS(tep)
	if tep.Status.GatewayIP != vrm.podIP {
		gatewayIP = vrm.getOverlayIP(tep.Status.GatewayIP)
		iFaceIndex = vrm.vxlanDevice.Link.Index
	} else {
		iFaceIndex = tep.Status.VethIFaceIndex
	}
	// Delete policy routing rule for the given cluster.
	klog.Infof("%s -> deleting policy routing rule for destination {%s} to lookup routing table with ID {%d}", clusterID, dstNet, vrm.routingTableID)
	if policyRuleDel, err = delPolicyRoutingRule("", dstNet, vrm.routingTableID); err != nil {
		return policyRuleDel, err
	}
	// Delete route for the given cluster.
	klog.Infof("%s -> deleting route for destination {%s} with gateway {%s} in routing table with ID {%d}",
		clusterID, dstNet, gatewayIP, vrm.routingTableID)
	routeDel, err = delRoute(dstNet, gatewayIP, iFaceIndex, vrm.routingTableID)
	if err != nil {
		return routeDel, err
	}
	if routeDel || policyRuleDel {
		configured = true
	}
	return configured, nil
}

// CleanRoutingTable removes all the routes from the custom routing table used by the route manager.
func (vrm *VxlanRoutingManager) CleanRoutingTable() error {
	return flushRoutesForRoutingTable(vrm.routingTableID)
}

// CleanPolicyRules removes all the policy rules pointing to the custom routing table used by the route manager.
func (vrm *VxlanRoutingManager) CleanPolicyRules() error {
	return flushRulesForRoutingTable(vrm.routingTableID)
}

func (vrm *VxlanRoutingManager) getOverlayIP(ip string) string {
	tokens := strings.Split(ip, ".")
	return strings.Join([]string{vrm.vxlanNetPrefix, tokens[1], tokens[2], tokens[3]}, ".")
}
