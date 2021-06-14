package routing

import (
	"strconv"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet"
	liqoerrors "github.com/liqotech/liqo/pkg/liqonet/errors"
)

// GatewayRoutingManager implements the routing manager interface.
// Used by the gateway operator to configure the routes for remote clusters.
type GatewayRoutingManager struct {
	routingTableID int
	tunnelDevice   netlink.Link
}

// NewGatewayRoutingManager returns a GatewayRoutingManager ready to be used or an error.
func NewGatewayRoutingManager(routingTableID int, tunnelDevice netlink.Link) (Routing, error) {
	// Check the validity of input parameters.
	if routingTableID > unix.RT_TABLE_MAX {
		return nil, &liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.MinorOrEqual + strconv.Itoa(unix.RT_TABLE_MAX)}
	}
	if routingTableID < 0 {
		return nil, &liqoerrors.WrongParameter{Parameter: "routingTableID", Reason: liqoerrors.GreaterOrEqual + strconv.Itoa(0)}
	}
	if tunnelDevice == nil {
		return nil, &liqoerrors.WrongParameter{
			Reason:    liqoerrors.NotNil,
			Parameter: "tunnelDevice",
		}
	}
	return &GatewayRoutingManager{
		routingTableID: routingTableID,
		tunnelDevice:   tunnelDevice,
	}, nil
}

// EnsureRoutesPerCluster accepts as input a netv1alpha.tunnelendpoint.
// It inserts the routes if they do not exist or updates them if they are outdated.
// Returns true if the routes have been configured, false if the routes are already configured.
// An error if something goes wrong and the routes can not be configured.
func (grm *GatewayRoutingManager) EnsureRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error) {
	var routeAdd bool
	var err error
	// Extract and save route information from the given tep.
	_, dstNet := liqonet.GetPodCIDRS(tep)
	// Add route for the given cluster.
	routeAdd, err = AddRoute(dstNet, "", grm.tunnelDevice.Attrs().Index, grm.routingTableID)
	if err != nil {
		return routeAdd, err
	}
	return routeAdd, nil
}

// RemoveRoutesPerCluster accepts as input a netv1alpha.tunnelendpoint.
// It deletes the routes if they do exist.
// Returns true if the routes exist and have been deleted, false if nothing is removed.
// An error if something goes wrong and the routes can not be removed.
func (grm *GatewayRoutingManager) RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error) {
	var routeDel bool
	var err error
	// Extract and save route information from the given tep.
	_, dstNet := liqonet.GetPodCIDRS(tep)
	// Delete route for the given cluster.
	routeDel, err = delRoute(dstNet, "", grm.tunnelDevice.Attrs().Index, grm.routingTableID)
	if err != nil {
		return routeDel, err
	}
	return routeDel, nil
}

// CleanRoutingTable stub function, as the gateway only operates in custom network namespace.
func (grm *GatewayRoutingManager) CleanRoutingTable() error {
	return flushRoutesForRoutingTable(grm.routingTableID)
}

// CleanPolicyRules stub function, as the gateway only operates in custom network namespace.
func (grm *GatewayRoutingManager) CleanPolicyRules() error {
	return flushRulesForRoutingTable(grm.routingTableID)
}
