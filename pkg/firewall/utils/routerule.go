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

package utils

import (
	"github.com/google/nftables"

	firewallv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

var _ Rule = &FilterRuleWrapper{}

// RouteRuleWrapper is a wrapper for a RouteRule.
type RouteRuleWrapper struct {
	*firewallv1beta1.RouteRule
}

// GetName returns the name of the rule.
func (rr *RouteRuleWrapper) GetName() *string {
	return rr.Name
}

// SetName sets the name of the rule.
func (rr *RouteRuleWrapper) SetName(name string) {
	rr.Name = &name
}

// Add adds the rule to the chain.
func (rr *RouteRuleWrapper) Add(_ *nftables.Conn, _ *nftables.Chain) error {
	// TODO: implement
	return nil
}

// Equal checks if the rule is equal to the given one.
func (rr *RouteRuleWrapper) Equal(_ *nftables.Rule) bool {
	// TODO: implement
	return true
}
