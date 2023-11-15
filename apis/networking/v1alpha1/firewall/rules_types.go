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
	"github.com/google/nftables/expr"
	"github.com/google/nftables/userdata"
)

// Rule is a rule to be applied to a chain.
type Rule interface {
	GetName() *string
	SetName(string)
	Add(nftconn *nftables.Conn, chain *nftables.Chain)
	Replace(nftconn *nftables.Conn, chain *nftables.Chain, handle uint64)
	Equal(rule *nftables.Rule) bool
}

var _ Rule = &NatRule{}

// NatRule is a rule to be applied to a NAT chain.
// +kubebuilder:object:generate=true
type NatRule struct {
	Name *string `json:"name,omitempty"`
}

// GetName returns the name of the rule.
func (nr *NatRule) GetName() *string {
	return nr.Name
}

// SetName sets the name of the rule.
func (nr *NatRule) SetName(name string) {
	nr.Name = &name
}

// Add adds the rule to the chain.
func (nr *NatRule) Add(nftconn *nftables.Conn, chain *nftables.Chain) {
	AddPlaceHolderRule(nftconn, chain, *nr.Name, 0)
}

// Replace replaces the rule in the chain.
func (nr *NatRule) Replace(nftconn *nftables.Conn, chain *nftables.Chain, handle uint64) {
	AddPlaceHolderRule(nftconn, chain, *nr.Name, handle)
}

// Equal checks if the rule is equal to the given one.
func (nr *NatRule) Equal(_ *nftables.Rule) bool {
	return true
}

var _ Rule = &FilterRule{}

// FilterRule is a rule to be applied to a filter chain.
// +kubebuilder:object:generate=true
type FilterRule struct {
	Name *string `json:"name,omitempty"`
}

// GetName returns the name of the rule.
func (fr *FilterRule) GetName() *string {
	return fr.Name
}

// SetName sets the name of the rule.
func (fr *FilterRule) SetName(name string) {
	fr.Name = &name
}

// Add adds the rule to the chain.
func (fr *FilterRule) Add(nftconn *nftables.Conn, chain *nftables.Chain) {
	AddPlaceHolderRule(nftconn, chain, *fr.Name, 0)
}

// Replace replaces the rule in the chain.
func (fr *FilterRule) Replace(nftconn *nftables.Conn, chain *nftables.Chain, handle uint64) {
	AddPlaceHolderRule(nftconn, chain, *fr.Name, handle)
}

// Equal checks if the rule is equal to the given one.
func (fr *FilterRule) Equal(_ *nftables.Rule) bool {
	return true
}

var _ Rule = &RouteRule{}

// RouteRule is a rule to be applied to a route chain.
// +kubebuilder:object:generate=true
type RouteRule struct {
	Name *string `json:"name,omitempty"`
}

// GetName returns the name of the rule.
func (rr *RouteRule) GetName() *string {
	return rr.Name
}

// SetName sets the name of the rule.
func (rr *RouteRule) SetName(name string) {
	rr.Name = &name
}

// Add adds the rule to the chain.
func (rr *RouteRule) Add(nftconn *nftables.Conn, chain *nftables.Chain) {
	AddPlaceHolderRule(nftconn, chain, *rr.Name, 0)
}

// Replace replaces the rule in the chain.
func (rr *RouteRule) Replace(nftconn *nftables.Conn, chain *nftables.Chain, handle uint64) {
	AddPlaceHolderRule(nftconn, chain, *rr.Name, handle)
}

// Equal checks if the rule is equal to the given one.
func (rr *RouteRule) Equal(_ *nftables.Rule) bool {
	return true
}

// RulesSet is a set of rules to be applied to a chain.
// +kubebuilder:object:generate=true
type RulesSet struct {
	// NatRules is a list of rules to be applied to the chain.
	NatRules []NatRule `json:"natRules,omitempty"`
	// FilterRules is a list of rules to be applied to the chain.
	FilterRules []FilterRule `json:"filterRules,omitempty"`
	// RouteRules is a list of rules to be applied to the chain.
	RouteRules []RouteRule `json:"routeRules,omitempty"`
}

// AddPlaceHolderRule adds a placeholder rule to the chain.
func AddPlaceHolderRule(nftconn *nftables.Conn, chain *nftables.Chain, name string, handle uint64) {
	nftconn.AddRule(&nftables.Rule{
		Table:    chain.Table,
		Chain:    chain,
		UserData: userdata.AppendString([]byte{}, userdata.TypeComment, name),
		Handle:   handle,
		Exprs: []expr.Any{
			&expr.Verdict{
				// [ immediate reg 0 drop ]
				Kind: expr.VerdictAccept,
			},
		},
	})
}
