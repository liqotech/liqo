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
	"k8s.io/utils/ptr"

	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	firewallutils "github.com/liqotech/liqo/pkg/firewall/utils"
)

func TestChainExpandOifFilterWithSetOperators(t *testing.T) {
	tests := []struct {
		name       string
		origOp     firewallapi.MatchOperation
		expectedOp firewallapi.MatchOperation
	}{
		{"eq becomes in", firewallapi.MatchOperationEq, firewallapi.MatchOperationIn},
		{"neq becomes nin", firewallapi.MatchOperationNeq, firewallapi.MatchOperationNin},
	}

	tunnels := []string{"liqo-tunnel", "liqo-tunnel1", "liqo-tunnel2"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &firewallapi.FilterRule{
				Name: ptr.To("mss-clamping-out"),
				Match: []firewallapi.Match{
					{
						Op: tt.origOp,
						Dev: &firewallapi.MatchDev{
							Value:    tunnelInterfaceName,
							Position: firewallapi.MatchDevPositionOut,
						},
					},
				},
				Action: firewallapi.ActionTCPMssClamp,
			}

			out := expandOifFilterWithSet(rule, tunnels)

			assert.NotSame(t, rule, out)
			assert.Len(t, out.Match, 1)
			assert.Nil(t, out.Match[0].Dev)
			assert.Equal(t, tt.expectedOp, out.Match[0].Op)
			if assert.NotNil(t, out.Match[0].Set) {
				assert.Equal(t, tunnels, out.Match[0].Set.Values)
				assert.Equal(t, firewallapi.MatchDevPositionOut, out.Match[0].Set.Position)
			}

			// The original rule must not be mutated (DeepCopy).
			assert.NotNil(t, rule.Match[0].Dev)
			assert.Nil(t, rule.Match[0].Set)
			assert.Equal(t, tt.origOp, rule.Match[0].Op)
		})
	}
}

func TestChainExpandOifFilterWithSetNoTunnels(t *testing.T) {
	tests := []struct {
		name    string
		tunnels []string
	}{
		{"nil tunnels", nil},
		{"empty tunnels", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &firewallapi.FilterRule{
				Name: ptr.To("mss-clamping-out"),
				Match: []firewallapi.Match{
					{
						Op: firewallapi.MatchOperationEq,
						Dev: &firewallapi.MatchDev{
							Value:    tunnelInterfaceName,
							Position: firewallapi.MatchDevPositionOut,
						},
					},
				},
				Action: firewallapi.ActionTCPMssClamp,
			}

			out := expandOifFilterWithSet(rule, tt.tunnels)

			assert.Same(t, rule, out)
			assert.NotNil(t, out.Match[0].Dev)
			assert.Nil(t, out.Match[0].Set)
			assert.Equal(t, firewallapi.MatchOperationEq, out.Match[0].Op)
		})
	}
}

func TestChainExpandOifFilterWithSetNoMatchingDev(t *testing.T) {
	tunnels := []string{"liqo-tunnel", "liqo-tunnel1", "liqo-tunnel2"}

	tests := []struct {
		name  string
		match firewallapi.Match
	}{
		{
			name: "dev with a different name",
			match: firewallapi.Match{
				Op: firewallapi.MatchOperationEq,
				Dev: &firewallapi.MatchDev{
					Value:    "eth0",
					Position: firewallapi.MatchDevPositionOut,
				},
			},
		},
		{
			name: "tunnel dev in input position",
			match: firewallapi.Match{
				Op: firewallapi.MatchOperationEq,
				Dev: &firewallapi.MatchDev{
					Value:    tunnelInterfaceName,
					Position: firewallapi.MatchDevPositionIn,
				},
			},
		},
		{
			name: "match without dev",
			match: firewallapi.Match{
				Op:    firewallapi.MatchOperationEq,
				Proto: &firewallapi.MatchProto{Value: firewallapi.L4ProtoTCP},
			},
		},
		{
			name: "dev already replaced by a set",
			match: firewallapi.Match{
				Op: firewallapi.MatchOperationIn,
				Set: &firewallapi.MatchSet{
					Values:   []string{"liqo-tunnel", "liqo-tunnel1"},
					Position: firewallapi.MatchDevPositionOut,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &firewallapi.FilterRule{
				Name:   ptr.To("some-rule"),
				Match:  []firewallapi.Match{tt.match},
				Action: firewallapi.ActionAccept,
			}
			expected := rule.DeepCopy()

			out := expandOifFilterWithSet(rule, tunnels)

			assert.NotSame(t, rule, out)
			assert.Equal(t, expected, out)
			// The input must not be mutated either.
			assert.Equal(t, expected, rule)
		})
	}
}

func TestChainExpandOifFilterWithSetMultipleMatches(t *testing.T) {
	tunnels := []string{"liqo-tunnel", "liqo-tunnel1", "liqo-tunnel2"}
	protoMatch := firewallapi.Match{
		Op:    firewallapi.MatchOperationEq,
		Proto: &firewallapi.MatchProto{Value: firewallapi.L4ProtoTCP},
	}
	devMatch := firewallapi.Match{
		Op: firewallapi.MatchOperationEq,
		Dev: &firewallapi.MatchDev{
			Value:    tunnelInterfaceName,
			Position: firewallapi.MatchDevPositionOut,
		},
	}

	tests := []struct {
		name     string
		matches  []firewallapi.Match
		devIndex int
	}{
		{"proto then dev", []firewallapi.Match{protoMatch, devMatch}, 1},
		{"dev then proto", []firewallapi.Match{devMatch, protoMatch}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &firewallapi.FilterRule{
				Name:    ptr.To("mss-clamping-out"),
				Match:   tt.matches,
				Counter: true,
				Action:  firewallapi.ActionTCPMssClamp,
			}
			original := rule.DeepCopy()

			out := expandOifFilterWithSet(rule, tunnels)

			// Same number and order of matches.
			assert.Len(t, out.Match, 2)

			// The dev match became a set match.
			expanded := out.Match[tt.devIndex]
			assert.Nil(t, expanded.Dev)
			assert.Equal(t, firewallapi.MatchOperationIn, expanded.Op)
			if assert.NotNil(t, expanded.Set) {
				assert.Equal(t, tunnels, expanded.Set.Values)
				assert.Equal(t, firewallapi.MatchDevPositionOut, expanded.Set.Position)
			}

			// The proto match is untouched.
			protoIndex := 1 - tt.devIndex
			assert.Equal(t, original.Match[protoIndex], out.Match[protoIndex])
			assert.Nil(t, out.Match[protoIndex].Set)

			// Counter and action are preserved.
			assert.True(t, out.Counter)
			assert.Equal(t, firewallapi.ActionTCPMssClamp, out.Action)

			// The input rule is not mutated.
			assert.Equal(t, original, rule)
		})
	}
}

func TestChainFromChainToRulesArrayTunnelThreshold(t *testing.T) {
	tests := []struct {
		name         string
		tunnels      []string
		expectExpand bool
	}{
		{"nil tunnels", nil, false},
		{"zero tunnels", []string{}, false},
		{"one tunnel", []string{"liqo-tunnel"}, false},
		{"two tunnels", []string{"liqo-tunnel", "liqo-tunnel1"}, true},
		{"three tunnels", []string{"liqo-tunnel", "liqo-tunnel1", "liqo-tunnel2"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := &firewallapi.Chain{
				Type: firewallapi.ChainTypeFilter,
				Rules: firewallapi.RulesSet{
					FilterRules: []firewallapi.FilterRule{
						{
							Name: ptr.To("mss-clamping-out"),
							Match: []firewallapi.Match{
								{
									Op: firewallapi.MatchOperationEq,
									Dev: &firewallapi.MatchDev{
										Value:    tunnelInterfaceName,
										Position: firewallapi.MatchDevPositionOut,
									},
								},
							},
							Action: firewallapi.ActionTCPMssClamp,
						},
					},
				},
			}

			rules := FromChainToRulesArray(chain, tt.tunnels)

			assert.Len(t, rules, 1)
			wrapper, ok := rules[0].(*firewallutils.FilterRuleWrapper)
			if !assert.True(t, ok) {
				return
			}
			m := wrapper.Match[0]

			if tt.expectExpand {
				assert.Nil(t, m.Dev)
				assert.Equal(t, firewallapi.MatchOperationIn, m.Op)
				if assert.NotNil(t, m.Set) {
					assert.Equal(t, tt.tunnels, m.Set.Values)
				}
			} else {
				assert.NotNil(t, m.Dev)
				assert.Nil(t, m.Set)
				assert.Equal(t, firewallapi.MatchOperationEq, m.Op)
			}

			// The chain passed by the caller is never mutated.
			assert.NotNil(t, chain.Rules.FilterRules[0].Match[0].Dev)
			assert.Nil(t, chain.Rules.FilterRules[0].Match[0].Set)
		})
	}
}

func TestChainFromChainToRulesArrayOrderAndSelectivity(t *testing.T) {
	tunnels := []string{"liqo-tunnel", "liqo-tunnel1", "liqo-tunnel2"}

	devRule := func(name, dev string, pos firewallapi.MatchDevPosition) firewallapi.FilterRule {
		return firewallapi.FilterRule{
			Name: ptr.To(name),
			Match: []firewallapi.Match{
				{
					Op:  firewallapi.MatchOperationEq,
					Dev: &firewallapi.MatchDev{Value: dev, Position: pos},
				},
			},
			Action: firewallapi.ActionAccept,
		}
	}

	chain := &firewallapi.Chain{
		Type: firewallapi.ChainTypeFilter,
		Rules: firewallapi.RulesSet{
			FilterRules: []firewallapi.FilterRule{
				devRule("first", "eth0", firewallapi.MatchDevPositionOut),
				devRule("second", tunnelInterfaceName, firewallapi.MatchDevPositionOut),
				devRule("third", tunnelInterfaceName, firewallapi.MatchDevPositionIn),
			},
		},
	}

	rules := FromChainToRulesArray(chain, tunnels)

	if !assert.Len(t, rules, 3) {
		return
	}

	names := make([]string, 0, len(rules))
	matches := make([]firewallapi.Match, 0, len(rules))
	for _, r := range rules {
		w, ok := r.(*firewallutils.FilterRuleWrapper)
		if !assert.True(t, ok) {
			return
		}
		names = append(names, *w.Name)
		matches = append(matches, w.Match[0])
	}

	// Same order as the input.
	assert.Equal(t, []string{"first", "second", "third"}, names)

	// Only the rule on "liqo-tunnel out" is expanded.
	assert.NotNil(t, matches[0].Dev)
	assert.Nil(t, matches[0].Set)

	assert.Nil(t, matches[1].Dev)
	if assert.NotNil(t, matches[1].Set) {
		assert.Equal(t, tunnels, matches[1].Set.Values)
	}
	assert.Equal(t, firewallapi.MatchOperationIn, matches[1].Op)

	assert.NotNil(t, matches[2].Dev)
	assert.Nil(t, matches[2].Set)
}
