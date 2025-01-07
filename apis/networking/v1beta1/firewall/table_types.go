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

// TableFamily specifies the family of the table.
type TableFamily string

// Possible TableFamily values.
// https://wiki.nftables.org/wiki-nftables/index.php/Nftables_families
const (
	TableFamilyINet   TableFamily = "INET"
	TableFamilyIPv4   TableFamily = "IPV4"
	TableFamilyIPv6   TableFamily = "IPV6"
	TableFamilyARP    TableFamily = "ARP"
	TableFamilyNetdev TableFamily = "NETDEV"
	TableFamilyBridge TableFamily = "BRIDGE"
)

// Table is a generic table to be applied to a chain.
// +kubebuilder:object:generate=true
type Table struct {
	// Name is the name of the table.
	Name *string `json:"name"`
	// Chains is a list of chains to be applied to the table.
	// +kubebuilder:validation:Optional
	Chains []Chain `json:"chains"`
	// Family is the family of the table.
	// +kubebuilder:validation:Enum="INET";"IPV4";"IPV6";"ARP";"NETDEV";"BRIDGE"
	Family *TableFamily `json:"family"`
}
