// Copyright 2019-2023 The Liqo Authors
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

// NatType is the type of the NAT rule.
type NatType string

const (
	// NatTypeDestination is the type of the NAT rule.
	NatTypeDestination NatType = "dnat"
	// NatTypeSource is the type of the NAT rule.
	NatTypeSource NatType = "snat"
	// NatTypeMasquerade is the type of the NAT rule.
	NatTypeMasquerade NatType = "masquerade"
)

var _ Rule = &NatRule{}

// NatRule is a rule to be applied to a NAT chain.
// +kubebuilder:object:generate=true
type NatRule struct {
	// Name is the name of the rule.
	Name *string `json:"name,omitempty"`
	// Match is the match to be applied to the rule.
	// They can be multiple and they are applied with an AND operator.
	// Using multiple ip matches with same position or
	Match []Match `json:"match"`
	// NatType is the type of the NAT rule.
	// +kubebuilder:validation:Enum=dnat;snat;masquerade
	NatType NatType `json:"natType"`
	// To is the IP to be used for the NAT translation.
	To *string `json:"to,omitempty"`
}
