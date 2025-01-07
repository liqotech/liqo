// Copyright 2019-2025 The Liqo Authors
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

package gateway

import (
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/gateway/concurrent"
	"github.com/liqotech/liqo/pkg/route"
)

const (
	// GatewayComponentKey is the key used to label the gateway pod.
	GatewayComponentKey = consts.NetworkingComponentKey

	// GatewayComponentGateway is the key used to label the gateway pod.
	GatewayComponentGateway = "gateway"

	// RouteCategoryGwTargetValue is the value used by the routeconfiguration controller to reconcile only resources related to a gateway.
	RouteCategoryGwTargetValue = "gateway"

	// RouteSubCategoryFabricTargetValue is the value used by the routeconfiguration controller to reconcile only resources related to a gateway.
	RouteSubCategoryFabricTargetValue = "fabric"
	// RouteSubCategoryFabricNodeTargetValue is the value used by the routeconfiguration controller
	// to reconcile only resources related to a gateway on anode.
	RouteSubCategoryFabricNodeTargetValue = "fabric-node"

	// FirewallCategoryGwTargetValue is the value used by the firewallconfiguration controller to reconcile only resources related to a gateway.
	FirewallCategoryGwTargetValue = "gateway"

	// FirewallSubCategoryFabricTargetValue is the value used by the firewallconfiguration controller to reconcile only resources related to a gateway.
	FirewallSubCategoryFabricTargetValue = "fabric"
)

// ForgeActiveGatewayPodLabels returns the labels for the gateway pod.
func ForgeActiveGatewayPodLabels() map[string]string {
	return map[string]string{
		concurrent.ActiveGatewayKey: concurrent.ActiveGatewayValue,
		GatewayComponentKey:         GatewayComponentGateway,
	}
}

// ForgeRouteExternalTargetLabels returns the labels used by the routeconfiguration controller
// to reconcile only resources related to a single gateway and external-network.
func ForgeRouteExternalTargetLabels(remoteID string) map[string]string {
	return map[string]string{
		route.RouteCategoryTargetKey: RouteCategoryGwTargetValue,
		route.RouteUniqueTargetKey:   remoteID,
	}
}

// ForgeRouteInternalTargetLabels returns the labels used by the routeconfiguration controller
// to reconcile only resources related to internal-network.
func ForgeRouteInternalTargetLabels() map[string]string {
	return map[string]string{
		route.RouteCategoryTargetKey:    RouteCategoryGwTargetValue,
		route.RouteSubCategoryTargetKey: RouteSubCategoryFabricTargetValue,
	}
}

// ForgeRouteInternalTargetLabelsByNode returns the labels used by the routeconfiguration controller
// to reconcile only resources related to internal-network and a single node.
func ForgeRouteInternalTargetLabelsByNode(nodeName string) map[string]string {
	return map[string]string{
		route.RouteCategoryTargetKey:    RouteCategoryGwTargetValue,
		route.RouteSubCategoryTargetKey: RouteSubCategoryFabricNodeTargetValue,
		route.RouteUniqueTargetKey:      nodeName,
	}
}

// ForgeFirewallInternalTargetLabels returns the labels used by the firewallconfiguration controller
// to reconcile only resources related to a gateway.
func ForgeFirewallInternalTargetLabels() map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryGwTargetValue,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryFabricTargetValue,
	}
}
