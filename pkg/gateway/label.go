// Copyright 2019-2024 The Liqo Authors
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
	"github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/route"
)

const (
	// GatewayComponentKey is the key used to label the gateway pod.
	GatewayComponentKey = "networking.liqo.io/component"

	// GatewayComponentGateway is the key used to label the gateway pod.
	GatewayComponentGateway = "gateway"

	// RouteCategoryTargetValue is the value used by the routeconfiguration controller to reconcile only resources related to a gateway.
	RouteCategoryTargetValue = "gateway"
	// RouteSubCategoryTargetValue is the value used by the routeconfiguration controller to reconcile only resources related to a gateway.
	RouteSubCategoryTargetValue = "fabric"

	// FirewallCategoryTargetValue is the value used by the firewallconfiguration controller to reconcile only resources related to a gateway.
	FirewallCategoryTargetValue = "gateway"
	// FirewallSubCategoryTargetValue is the value used by the firewallconfiguration controller to reconcile only resources related to a gateway.
	FirewallSubCategoryTargetValue = "fabric"
)

// ForgeGatewayPodLabels returns the labels for the gateway pod.
func ForgeGatewayPodLabels() map[string]string {
	return map[string]string{
		GatewayComponentKey: GatewayComponentGateway,
	}
}

// ForgeRouteExternalTargetLabels returns the labels used by the routeconfiguration controller
// to reconcile only resources related to a single gateway and external-network.
func ForgeRouteExternalTargetLabels(remoteID string) map[string]string {
	return map[string]string{
		route.RouteCategoryTargetKey: RouteCategoryTargetValue,
		route.RouteUniqueTargetKey:   remoteID,
	}
}

// ForgeRouteInternalTargetLabels returns the labels used by the routeconfiguration controller
// to reconcile only resources related to internal-network.
func ForgeRouteInternalTargetLabels() map[string]string {
	return map[string]string{
		route.RouteCategoryTargetKey:    RouteCategoryTargetValue,
		route.RouteSubCategoryTargetKey: RouteSubCategoryTargetValue,
	}
}

// ForgeFirewallInternalTargetLabels returns the labels used by the firewallconfiguration controller
// to reconcile only resources related to a gateway.
func ForgeFirewallInternalTargetLabels() map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValue,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValue,
	}
}
