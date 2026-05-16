// Copyright 2019-2026 The Liqo Authors
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

package remapping

import (
	"github.com/liqotech/liqo/pkg/firewall"
)

const (
	// FirewallCategoryTargetValueGw is the value used by the firewallconfiguration controller to reconcile only resources related to a gateway.
	FirewallCategoryTargetValueGw = "gateway"
	// FirewallCategoryTargetValueFabric is the value used by the firewallconfiguration controller to reconcile only resources related to fabric.
	FirewallCategoryTargetValueFabric = "fabric"
	// FirewallSubCategoryTargetValueIPMapping is the value used by the firewallconfiguration controller
	// to reconcile only resources related to the IP mapping.
	FirewallSubCategoryTargetValueIPMapping = "ip-mapping"
)

// ForgeFirewallTargetLabels returns the labels used by the firewallconfiguration controller
// to reconcile only resources related to a single gateway.
func ForgeFirewallTargetLabels(remoteID string) map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey: FirewallCategoryTargetValueGw,
		firewall.FirewallUniqueTargetKey:   remoteID,
	}
}

// ForgeFirewallTargetLabelsIPMappingGw returns the labels used by the firewallconfiguration
// controller to reconcile only resources related to the IP mapping.
func ForgeFirewallTargetLabelsIPMappingGw() map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueGw,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
	}
}

// ForgeFirewallTargetLabelsIPMappingFabric returns the labels used by the firewallconfiguration
// controller to reconcile only resources related to the IP mapping.
func ForgeFirewallTargetLabelsIPMappingFabric() map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueFabric,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
	}
}

// ForgeFirewallBindingTargetLabels returns the labels used by the firewallconfigurationbinding controller
// to reconcile only resources related to a single gateway, for the given gateway.
// The remoteID is stored as subcategory to preserve the remote cluster identity alongside the gateway name.
func ForgeFirewallBindingTargetLabels(remoteID, gatewayName string) map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueGw,
		firewall.FirewallSubCategoryTargetKey: remoteID,
		firewall.FirewallUniqueTargetKey:      gatewayName,
	}
}

// ForgeFirewallBindingTargetLabelsIPMappingGw returns the labels used by the firewallconfigurationbinding
// controller to reconcile only resources related to the IP mapping for a specific gateway.
func ForgeFirewallBindingTargetLabelsIPMappingGw(gatewayName string) map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueGw,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
		firewall.FirewallUniqueTargetKey:      gatewayName,
	}
}

// ForgeFirewallBindingTargetLabelsIPMappingFabric returns the labels used by the firewallconfigurationbinding
// controller to reconcile only resources related to the IP mapping for a specific fabric node.
func ForgeFirewallBindingTargetLabelsIPMappingFabric(nodeName string) map[string]string {
	return map[string]string{
		firewall.FirewallCategoryTargetKey:    FirewallCategoryTargetValueFabric,
		firewall.FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
		firewall.FirewallUniqueTargetKey:      nodeName,
	}
}
