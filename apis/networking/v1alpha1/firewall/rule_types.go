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

import (
	"github.com/google/nftables"
)

// Rule is a rule to be applied to a chain.
type Rule interface {
	GetName() *string
	SetName(string)
	Add(nftconn *nftables.Conn, chain *nftables.Chain) error
	Equal(rule *nftables.Rule) bool
}

// RulesSet is a set of rules to be applied to a chain.
// +kubebuilder:object:generate=true
// +kubebuilder:validation:MaxProperties=1
// +kubebuilder:validation:MinProperties=1
type RulesSet struct {
	// NatRules is a list of rules to be applied to the chain.
	NatRules []NatRule `json:"natRules,omitempty"`
	// FilterRules is a list of rules to be applied to the chain.
	FilterRules []FilterRule `json:"filterRules,omitempty"`
	// RouteRules is a list of rules to be applied to the chain.
	RouteRules []RouteRule `json:"routeRules,omitempty"`
}
