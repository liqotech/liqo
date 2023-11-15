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

// NatRule is a rule to be applied to a NAT chain.
// +kubebuilder:object:generate=true
type NatRule struct {
}

// FilterRule is a rule to be applied to a filter chain.
// +kubebuilder:object:generate=true
type FilterRule struct {
}

// RouteRule is a rule to be applied to a route chain.
// +kubebuilder:object:generate=true
type RouteRule struct {
}

// RulesSet is a set of rules to be applied to a chain.
// +kubebuilder:object:generate=true
type RulesSet struct {
	// NatRules is a list of rules to be applied to the chain.
	// +kubebuilder:validation:Optional
	NatRules []NatRule `json:"natRules"`
	// FilterRules is a list of rules to be applied to the chain.
	// +kubebuilder:validation:Optional
	FilterRules []FilterRule `json:"filterRules"`
	// RouteRules is a list of rules to be applied to the chain.
	// +kubebuilder:validation:Optional
	RouteRules []RouteRule `json:"routeRules"`
}
