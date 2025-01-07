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

package fabric

import (
	"github.com/liqotech/liqo/pkg/firewall"
	"github.com/liqotech/liqo/pkg/route"
)

const (
	// FirewallCategoryTargetValue is the value used by the firewallconfiguration controller
	// to reconcile only resources related to netwok fabric.
	FirewallCategoryTargetValue = "fabric"
	// FirewallSubCategoryTargetAllNodesValue is the value used by the firewallconfiguration controller
	// to reconcile only resources related to netwok fabric on all nodes.
	FirewallSubCategoryTargetAllNodesValue = "all-nodes"
	// FirewallSubCategoryTargetSingleNodeValue is the value used by the firewallconfiguration controller
	// to reconcile only resources related to netwok fabric on a specific node.
	FirewallSubCategoryTargetSingleNodeValue = "single-node"
	// RouteCategoryTargetValue is the value used by the routecontroller to reconcile only resources related to network fabric.
	RouteCategoryTargetValue = "fabric"
)

// ForgeFirewallTargetLabels returns the labels used by the firewallconfiguration controller
// to reconcile only resources related to network fabric.
func ForgeFirewallTargetLabels() map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValue,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetAllNodesValue,
	}
}

// ForgeFirewallTargetLabelsSingleNode returns the labels used by the firewallconfiguration controller
// to reconcile only resources related to network fabric on a specific node.
func ForgeFirewallTargetLabelsSingleNode(nodeName string) map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValue,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetSingleNodeValue,
		firewall.FirewallUniqueTargetKey:      nodeName,
	}
}

// ForgeRouteTargetLabels returns the labels used by the routecontroller
// to reconcile only resources related to network fabric.
func ForgeRouteTargetLabels() map[string]string {
	return map[string]string{
		route.RouteCategoryTargetKey: RouteCategoryTargetValue,
	}
}
