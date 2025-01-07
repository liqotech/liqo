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

package firewall

const (
	// FirewallCategoryTargetKey is the key used by the firewallconfiguration controller to reconcile only resources related to a group.
	FirewallCategoryTargetKey = "networking.liqo.io/firewall-category"
	// FirewallSubCategoryTargetKey is the key used by the firewallconfiguration controller to reconcile only resources related to a subgroup.
	FirewallSubCategoryTargetKey = "networking.liqo.io/firewall-subcategory"
	// FirewallUniqueTargetKey is the key used by the firewallconfiguration controller to reconcile only resources related to a single component.
	FirewallUniqueTargetKey = "networking.liqo.io/firewall-unique"
)
