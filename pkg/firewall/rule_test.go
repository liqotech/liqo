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

import (
	"testing"

	"github.com/stretchr/testify/assert"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	firewallutils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func TestRuleGetMatchesFromRule(t *testing.T) {
	setMatch := firewallapi.Match{
		Op: firewallapi.MatchOperationIn,
		Set: &firewallapi.MatchSet{
			Values:   []string{"liqo-tunnel", "liqo-tunnel1"},
			Position: firewallapi.MatchDevPositionOut,
		},
	}
	markMatch := firewallapi.Match{
		Op:   firewallapi.MatchOperationEq,
		Mark: &firewallapi.MatchMark{Value: "65280"},
	}

	tests := []struct {
		name     string
		rule     firewallutils.Rule
		expected []firewallapi.Match
	}{
		{
			name: "filter rule",
			rule: &firewallutils.FilterRuleWrapper{
				FilterRule: &firewallapi.FilterRule{
					Match: []firewallapi.Match{setMatch, markMatch},
				},
			},
			expected: []firewallapi.Match{setMatch, markMatch},
		},
		{
			name: "nat rule",
			rule: &firewallutils.NatRuleWrapper{
				NatRule: &firewallapi.NatRule{
					Match: []firewallapi.Match{markMatch},
				},
			},
			expected: []firewallapi.Match{markMatch},
		},
		{
			name:     "filter rule without matches",
			rule:     &firewallutils.FilterRuleWrapper{FilterRule: &firewallapi.FilterRule{}},
			expected: nil,
		},
		{
			name:     "route rule",
			rule:     &firewallutils.RouteRuleWrapper{RouteRule: &firewallapi.RouteRule{}},
			expected: nil,
		},
		{
			name:     "nil rule",
			rule:     nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, getMatchesFromRule(tt.rule))
		})
	}
}
