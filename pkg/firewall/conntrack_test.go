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
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ti-mo/conntrack"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

const (
	testSrcIP = "10.0.0.1"
	testDstIP = "10.0.0.2"
)

type fakeConntrackClient struct {
	flows       []conntrack.Flow
	dumpErr     error
	deleteErr   error
	deleted     []conntrack.Flow
	closeCalled bool
}

func (f *fakeConntrackClient) Dump(_ *conntrack.DumpOptions) ([]conntrack.Flow, error) {
	return f.flows, f.dumpErr
}

func (f *fakeConntrackClient) Delete(flow *conntrack.Flow) error {
	f.deleted = append(f.deleted, *flow)
	return f.deleteErr
}

func (f *fakeConntrackClient) Close() error {
	f.closeCalled = true
	return nil
}

func newFlow(proto uint8, srcAddr, dstAddr string, srcPort, dstPort uint16) conntrack.Flow {
	return conntrack.NewFlow(
		proto, 0,
		netip.MustParseAddr(srcAddr),
		netip.MustParseAddr(dstAddr),
		srcPort, dstPort, 120, 0,
	)
}

func TestNotrackRules(t *testing.T) {
	tests := []struct {
		name     string
		table    *firewallapi.Table
		expected []firewallapi.FilterRule
	}{
		{
			name:     "nil table",
			table:    nil,
			expected: nil,
		},
		{
			name: "no notrack rules",
			table: &firewallapi.Table{
				Chains: []firewallapi.Chain{
					{
						Rules: firewallapi.RulesSet{
							FilterRules: []firewallapi.FilterRule{
								{Action: firewallapi.ActionAccept},
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "notrack rule present",
			table: &firewallapi.Table{
				Chains: []firewallapi.Chain{
					{
						Rules: firewallapi.RulesSet{
							FilterRules: []firewallapi.FilterRule{
								{Action: firewallapi.ActionNotrack},
							},
						},
					},
				},
			},
			expected: []firewallapi.FilterRule{{Action: firewallapi.ActionNotrack}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, notrackRules(tt.table))
		})
	}
}

func TestRuleMatchesFlow(t *testing.T) {
	flow := newFlow(6, testSrcIP, testDstIP, 12345, 80)

	tests := []struct {
		name     string
		rule     firewallapi.FilterRule
		flow     conntrack.Flow
		expected bool
	}{
		{
			name:     "empty rule matches any flow",
			rule:     firewallapi.FilterRule{Action: firewallapi.ActionNotrack},
			flow:     flow,
			expected: true,
		},
		{
			name: "matches source IP",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op: firewallapi.MatchOperationEq,
						IP: &firewallapi.MatchIP{Value: testSrcIP, Position: firewallapi.MatchPositionSrc},
					},
				},
			},
			flow:     flow,
			expected: true,
		},
		{
			name: "matches destination IP in reply direction",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op: firewallapi.MatchOperationEq,
						IP: &firewallapi.MatchIP{Value: testSrcIP, Position: firewallapi.MatchPositionDst},
					},
				},
			},
			flow:     flow,
			expected: true,
		},
		{
			name: "does not match unrelated source IP",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op: firewallapi.MatchOperationEq,
						IP: &firewallapi.MatchIP{Value: "192.168.1.1", Position: firewallapi.MatchPositionSrc},
					},
				},
			},
			flow:     flow,
			expected: false,
		},
		{
			name: "matches source port",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op:   firewallapi.MatchOperationEq,
						Port: &firewallapi.MatchPort{Value: "12345", Position: firewallapi.MatchPositionSrc},
					},
				},
			},
			flow:     flow,
			expected: true,
		},
		{
			name: "matches port range",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op:   firewallapi.MatchOperationEq,
						Port: &firewallapi.MatchPort{Value: "70-90", Position: firewallapi.MatchPositionDst},
					},
				},
			},
			flow:     flow,
			expected: true,
		},
		{
			name: "matches protocol",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op:    firewallapi.MatchOperationEq,
						Proto: &firewallapi.MatchProto{Value: firewallapi.L4ProtoTCP},
					},
				},
			},
			flow:     flow,
			expected: true,
		},
		{
			name: "neq matches flow in reply direction",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op: firewallapi.MatchOperationNeq,
						IP: &firewallapi.MatchIP{Value: testSrcIP, Position: firewallapi.MatchPositionSrc},
					},
				},
			},
			flow:     flow,
			expected: true,
		},
		{
			name: "matches CIDR",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op: firewallapi.MatchOperationEq,
						IP: &firewallapi.MatchIP{Value: "10.0.0.0/24", Position: firewallapi.MatchPositionSrc},
					},
				},
			},
			flow:     flow,
			expected: true,
		},
		{
			name: "matches all fields in single Match",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op:    firewallapi.MatchOperationEq,
						IP:    &firewallapi.MatchIP{Value: testSrcIP, Position: firewallapi.MatchPositionSrc},
						Port:  &firewallapi.MatchPort{Value: "12345", Position: firewallapi.MatchPositionSrc},
						Proto: &firewallapi.MatchProto{Value: firewallapi.L4ProtoTCP},
					},
				},
			},
			flow:     flow,
			expected: true,
		},
		{
			name: "fails when one field in single Match does not match",
			rule: firewallapi.FilterRule{
				Action: firewallapi.ActionNotrack,
				Match: []firewallapi.Match{
					{
						Op:    firewallapi.MatchOperationEq,
						IP:    &firewallapi.MatchIP{Value: testSrcIP, Position: firewallapi.MatchPositionSrc},
						Port:  &firewallapi.MatchPort{Value: "99999", Position: firewallapi.MatchPositionSrc},
						Proto: &firewallapi.MatchProto{Value: firewallapi.L4ProtoTCP},
					},
				},
			},
			flow:     flow,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ruleMatchesFlow(&tt.rule, &tt.flow))
		})
	}
}

func TestParsePortRange(t *testing.T) {
	tests := []struct {
		value      string
		start, end uint16
		ok         bool
	}{
		{"80", 80, 80, true},
		{"3000-4000", 3000, 4000, true},
		{"0-65535", 0, 65535, true},
		{"abc", 0, 0, false},
		{"80-70", 0, 0, false},
		{"70000", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			start, end, ok := parsePortRange(tt.value)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.start, start)
				assert.Equal(t, tt.end, end)
			}
		})
	}
}

func TestFlushConntrackForNotrackRules(t *testing.T) {
	matchingFlow := newFlow(6, testSrcIP, testDstIP, 12345, 80)
	nonMatchingFlow := newFlow(17, "192.168.1.1", "192.168.1.2", 53, 53)

	rule := firewallapi.FilterRule{
		Action: firewallapi.ActionNotrack,
		Match: []firewallapi.Match{
			{
				Op: firewallapi.MatchOperationEq,
				IP: &firewallapi.MatchIP{Value: testSrcIP, Position: firewallapi.MatchPositionSrc},
			},
			{
				Op:    firewallapi.MatchOperationEq,
				Proto: &firewallapi.MatchProto{Value: firewallapi.L4ProtoTCP},
			},
		},
	}

	fwcfg := &networkingv1beta1.FirewallConfiguration{
		Spec: networkingv1beta1.FirewallConfigurationSpec{
			Table: firewallapi.Table{
				Chains: []firewallapi.Chain{
					{
						Rules: firewallapi.RulesSet{
							FilterRules: []firewallapi.FilterRule{rule},
						},
					},
				},
			},
		},
	}

	t.Run("deletes only matching flows", func(t *testing.T) {
		fake := &fakeConntrackClient{flows: []conntrack.Flow{matchingFlow, nonMatchingFlow}}
		r := &FirewallConfigurationReconciler{ConntrackClient: fake}
		err := r.flushConntrackForNotrackRules(context.Background(), fwcfg)
		assert.NoError(t, err)
		assert.Len(t, fake.deleted, 1)
		assert.Equal(t, matchingFlow, fake.deleted[0])
	})

	t.Run("no notrack rules does nothing", func(t *testing.T) {
		fake := &fakeConntrackClient{flows: []conntrack.Flow{matchingFlow}}
		r := &FirewallConfigurationReconciler{ConntrackClient: fake}
		cfg := &networkingv1beta1.FirewallConfiguration{
			Spec: networkingv1beta1.FirewallConfigurationSpec{
				Table: firewallapi.Table{
					Chains: []firewallapi.Chain{
						{
							Rules: firewallapi.RulesSet{
								FilterRules: []firewallapi.FilterRule{{Action: firewallapi.ActionAccept}},
							},
						},
					},
				},
			},
		}
		err := r.flushConntrackForNotrackRules(context.Background(), cfg)
		assert.NoError(t, err)
		assert.Empty(t, fake.deleted)
	})

	t.Run("no conntrack client configured", func(t *testing.T) {
		r := &FirewallConfigurationReconciler{}
		err := r.flushConntrackForNotrackRules(context.Background(), fwcfg)
		assert.NoError(t, err)
	})

	t.Run("dump error is propagated", func(t *testing.T) {
		fake := &fakeConntrackClient{dumpErr: errors.New("dump failed")}
		r := &FirewallConfigurationReconciler{ConntrackClient: fake}
		err := r.flushConntrackForNotrackRules(context.Background(), fwcfg)
		assert.Error(t, err)
	})

	t.Run("delete error is propagated", func(t *testing.T) {
		fake := &fakeConntrackClient{flows: []conntrack.Flow{matchingFlow}, deleteErr: errors.New("delete failed")}
		r := &FirewallConfigurationReconciler{ConntrackClient: fake}
		err := r.flushConntrackForNotrackRules(context.Background(), fwcfg)
		assert.Error(t, err)
	})
}

func TestMatchIPInvalidValue(t *testing.T) {
	tuple := &conntrack.Tuple{
		IP: conntrack.IPTuple{
			SourceAddress:      netip.MustParseAddr(testSrcIP),
			DestinationAddress: netip.MustParseAddr(testDstIP),
		},
	}
	match := &firewallapi.MatchIP{Value: "not-an-ip", Position: firewallapi.MatchPositionSrc}
	assert.False(t, matchIP(tuple, match))
}
