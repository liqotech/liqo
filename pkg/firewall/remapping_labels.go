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

package firewall

const (
	// FirewallCategoryTargetValueGw is the category value used to target gateway entities.
	FirewallCategoryTargetValueGw = "gateway"
	// FirewallCategoryTargetValueFabric is the category value used to target fabric node entities.
	FirewallCategoryTargetValueFabric = "fabric"
	// FirewallSubCategoryTargetValueIPMapping is the value used by the firewallconfiguration controller
	// to reconcile only resources related to the IP mapping.
	FirewallSubCategoryTargetValueIPMapping = "ip-mapping"
)

// ForgeFirewallBindingTargetLabelsRemapping returns the labels used by the firewallconfigurationbinding controller
// to reconcile only resources related to a single gateway, for the given gateway.
// The remoteID is stored as subcategory to preserve the remote cluster identity alongside the gateway name.
func ForgeFirewallBindingTargetLabelsRemapping(remoteID, gatewayName string) map[string]string {
	return map[string]string{
		FirewallCategoryTargetKey:    FirewallCategoryTargetValueGw,
		FirewallSubCategoryTargetKey: remoteID,
		FirewallUniqueTargetKey:      gatewayName,
	}
}

// ForgeFirewallBindingTargetLabelsIPMappingGw returns the labels used by the firewallconfigurationbinding
// controller to reconcile only resources related to the IP mapping for a specific gateway.
func ForgeFirewallBindingTargetLabelsIPMappingGw(gatewayName string) map[string]string {
	return map[string]string{
		FirewallCategoryTargetKey:    FirewallCategoryTargetValueGw,
		FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
		FirewallUniqueTargetKey:      gatewayName,
	}
}

// ForgeFirewallBindingTargetLabelsIPMappingFabric returns the labels used by the firewallconfigurationbinding
// controller to reconcile only resources related to the IP mapping for a specific fabric node.
func ForgeFirewallBindingTargetLabelsIPMappingFabric(nodeName string) map[string]string {
	return map[string]string{
		FirewallCategoryTargetKey:    FirewallCategoryTargetValueFabric,
		FirewallSubCategoryTargetKey: FirewallSubCategoryTargetValueIPMapping,
		FirewallUniqueTargetKey:      nodeName,
	}
}
