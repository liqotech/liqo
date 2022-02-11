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
	"strconv"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoerrors "github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
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
	var routePodCIDRAdd, routeExternalCIDRAdd, configured bool
	var err error
	// Extract and save route information from the given tep.
	_, dstPodCIDRNet := utils.GetPodCIDRS(tep)
	_, dstExternalCIDRNet := utils.GetExternalCIDRS(tep)
	// Add routes for the given cluster.
	routePodCIDRAdd, err = AddRoute(dstPodCIDRNet, "", grm.tunnelDevice.Attrs().Index, grm.routingTableID, DefaultFlags, DefaultScope)
	if err != nil {
		return routePodCIDRAdd, err
	}
	routeExternalCIDRAdd, err = AddRoute(dstExternalCIDRNet, "", grm.tunnelDevice.Attrs().Index, grm.routingTableID, DefaultFlags, DefaultScope)
	if err != nil {
		return routeExternalCIDRAdd, err
	}
	if routePodCIDRAdd || routeExternalCIDRAdd {
		configured = true
	}
	return configured, nil
}

// RemoveRoutesPerCluster accepts as input a netv1alpha.tunnelendpoint.
// It deletes the routes if they do exist.
// Returns true if the routes exist and have been deleted, false if nothing is removed.
// An error if something goes wrong and the routes can not be removed.
func (grm *GatewayRoutingManager) RemoveRoutesPerCluster(tep *netv1alpha1.TunnelEndpoint) (bool, error) {
	var routePodCIDRDel, routeExternalCIDRDel, configured bool
	var err error
	// Extract and save route information from the given tep.
	_, dstPodCIDRNet := utils.GetPodCIDRS(tep)
	_, dstExternalCIDRNet := utils.GetExternalCIDRS(tep)
	// Delete routes for the given cluster.
	routePodCIDRDel, err = DelRoute(dstPodCIDRNet, "", grm.tunnelDevice.Attrs().Index, grm.routingTableID)
	if err != nil {
		return routePodCIDRDel, err
	}
	routeExternalCIDRDel, err = DelRoute(dstExternalCIDRNet, "", grm.tunnelDevice.Attrs().Index, grm.routingTableID)
	if err != nil {
		return routeExternalCIDRDel, err
	}
	if routePodCIDRDel || routeExternalCIDRDel {
		configured = true
	}
	return configured, nil
}

// CleanRoutingTable stub function, as the gateway only operates in custom network namespace.
func (grm *GatewayRoutingManager) CleanRoutingTable() error {
	return flushRoutesForRoutingTable(grm.routingTableID)
}

// CleanPolicyRules stub function, as the gateway only operates in custom network namespace.
func (grm *GatewayRoutingManager) CleanPolicyRules() error {
	return flushRulesForRoutingTable(grm.routingTableID)
}
