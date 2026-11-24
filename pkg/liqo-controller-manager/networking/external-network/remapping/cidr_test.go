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

package remapping

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/utils"
)

// newTestConfiguration returns a Configuration with the pointer fields
// initialized, so that tests can write into them without nil dereferences.
func newTestConfiguration() *networkingv1beta1.Configuration {
	cfg := &networkingv1beta1.Configuration{}
	cfg.Spec.Local = &networkingv1beta1.ClusterConfig{}
	cfg.Status.Remote = &networkingv1beta1.ClusterConfig{}
	return cfg
}

func TestCIDRDNATRulesUseGwExtMark(t *testing.T) {
	tests := []struct {
		name     string
		cidrType CIDRType
		fill     func(cfg *networkingv1beta1.Configuration)
	}{
		{
			name:     "pod cidr",
			cidrType: PodCIDR,
			fill: func(cfg *networkingv1beta1.Configuration) {
				cfg.Spec.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.200.0.0/16", "10.100.0.0/16"}
				cfg.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.61.0.0/16", "10.100.0.0/16"}
				// Decoy: a remapped pair in the other list must be ignored.
				cfg.Spec.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.50.0.0/16"}
				cfg.Status.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.51.0.0/16"}
			},
		},
		{
			name:     "external cidr",
			cidrType: ExternalCIDR,
			fill: func(cfg *networkingv1beta1.Configuration) {
				cfg.Spec.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.200.0.0/16", "10.100.0.0/16"}
				cfg.Status.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.61.0.0/16", "10.100.0.0/16"}
				// Decoy: a remapped pair in the other list must be ignored.
				cfg.Spec.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.50.0.0/16"}
				cfg.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.51.0.0/16"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfiguration()
			tt.fill(cfg)

			rules := forgeCIDRFirewallConfigurationDNATRules(cfg, &Options{}, tt.cidrType)

			// Only the remapped pair of the selected type produces a rule.
			if !assert.Len(t, rules, 1) {
				return
			}
			rule := rules[0]

			assert.Equal(t, firewall.NatTypeDestination, rule.NatType)
			assert.Equal(t, ptr.To("10.200.0.0/16"), rule.To)

			if !assert.Len(t, rule.Match, 2) {
				return
			}

			assert.Equal(t, firewall.MatchOperationEq, rule.Match[0].Op)
			if assert.NotNil(t, rule.Match[0].IP) {
				assert.Equal(t, "10.61.0.0/16", rule.Match[0].IP.Value)
				assert.Equal(t, firewall.MatchPositionDst, rule.Match[0].IP.Position)
			}

			assert.Equal(t, firewall.MatchOperationEq, rule.Match[1].Op)
			if assert.NotNil(t, rule.Match[1].Mark) {
				assert.Equal(t, fmt.Sprintf("%d", utils.GwExtMark), rule.Match[1].Mark.Value)
			}

			// No interface match is left in the DNAT rule.
			for i := range rule.Match {
				assert.Nil(t, rule.Match[i].Dev)
			}
		})
	}
}

func TestCIDRSNATFirstRuleUsesGwNodeMark(t *testing.T) {
	tests := []struct {
		name     string
		cidrType CIDRType
		fill     func(cfg *networkingv1beta1.Configuration)
	}{
		{
			name:     "pod cidr",
			cidrType: PodCIDR,
			fill: func(cfg *networkingv1beta1.Configuration) {
				cfg.Spec.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.200.0.0/16", "10.100.0.0/16"}
				cfg.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.61.0.0/16", "10.100.0.0/16"}
				// Decoy: a remapped pair in the other list must be ignored.
				cfg.Spec.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.50.0.0/16"}
				cfg.Status.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.51.0.0/16"}
			},
		},
		{
			name:     "external cidr",
			cidrType: ExternalCIDR,
			fill: func(cfg *networkingv1beta1.Configuration) {
				cfg.Spec.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.200.0.0/16", "10.100.0.0/16"}
				cfg.Status.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.61.0.0/16", "10.100.0.0/16"}
				cfg.Spec.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.50.0.0/16"}
				cfg.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.51.0.0/16"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfiguration()
			cfg.Spec.Local.CIDR.External = []networkingv1beta1.CIDR{"10.70.0.0/16"}
			tt.fill(cfg)

			rules := forgeCIDRFirewallConfigurationSNATRules(cfg, &Options{DefaultInterfaceName: "eth0"}, tt.cidrType)

			// Two rules for the only remapped pair, none for the identical one.
			if !assert.Len(t, rules, 2) {
				return
			}
			rule := rules[0]

			assert.Equal(t, firewall.NatTypeSource, rule.NatType)
			assert.Equal(t, ptr.To("10.61.0.0/16"), rule.To)

			if !assert.Len(t, rule.Match, 3) {
				return
			}

			assert.Equal(t, firewall.MatchOperationNeq, rule.Match[0].Op)
			if assert.NotNil(t, rule.Match[0].Dev) {
				assert.Equal(t, "eth0", rule.Match[0].Dev.Value)
				assert.Equal(t, firewall.MatchDevPositionOut, rule.Match[0].Dev.Position)
			}

			assert.Equal(t, firewall.MatchOperationEq, rule.Match[1].Op)
			if assert.NotNil(t, rule.Match[1].IP) {
				assert.Equal(t, "10.200.0.0/16", rule.Match[1].IP.Value)
				assert.Equal(t, firewall.MatchPositionSrc, rule.Match[1].IP.Position)
			}

			assert.Equal(t, firewall.MatchOperationEq, rule.Match[2].Op)
			if assert.NotNil(t, rule.Match[2].Mark) {
				assert.Equal(t, fmt.Sprintf("%d", utils.GwNodeMark), rule.Match[2].Mark.Value)
			}
		})
	}
}

func TestCIDRSNATSecondRuleExcludesGeneveTraffic(t *testing.T) {
	tests := []struct {
		name     string
		cidrType CIDRType
		fill     func(cfg *networkingv1beta1.Configuration)
	}{
		{
			name:     "pod cidr",
			cidrType: PodCIDR,
			fill: func(cfg *networkingv1beta1.Configuration) {
				cfg.Spec.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.200.0.0/16", "10.100.0.0/16"}
				cfg.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.61.0.0/16", "10.100.0.0/16"}
			},
		},
		{
			name:     "external cidr",
			cidrType: ExternalCIDR,
			fill: func(cfg *networkingv1beta1.Configuration) {
				cfg.Spec.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.200.0.0/16", "10.100.0.0/16"}
				cfg.Status.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.61.0.0/16", "10.100.0.0/16"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfiguration()
			cfg.Spec.Local.CIDR.External = []networkingv1beta1.CIDR{"10.70.0.0/16"}
			tt.fill(cfg)

			unknownIP := getUnknownSourceIPFromConfiguration(cfg)
			// A valid local External CIDR must yield a usable IP.
			if !assert.NotEmpty(t, unknownIP) {
				return
			}

			rules := forgeCIDRFirewallConfigurationSNATRules(cfg, &Options{DefaultInterfaceName: "eth0"}, tt.cidrType)
			if !assert.Len(t, rules, 2) {
				return
			}
			rule := rules[1]

			assert.Equal(t, firewall.NatTypeSource, rule.NatType)
			assert.Equal(t, ptr.To("10.61.0.0/16"), rule.To)

			if !assert.Len(t, rule.Match, 4) {
				return
			}

			assert.Equal(t, firewall.MatchOperationNeq, rule.Match[0].Op)
			if assert.NotNil(t, rule.Match[0].Dev) {
				assert.Equal(t, "eth0", rule.Match[0].Dev.Value)
				assert.Equal(t, firewall.MatchDevPositionOut, rule.Match[0].Dev.Position)
			}

			assert.Equal(t, firewall.MatchOperationEq, rule.Match[1].Op)
			if assert.NotNil(t, rule.Match[1].IP) {
				assert.Equal(t, "10.200.0.0/16", rule.Match[1].IP.Value)
				assert.Equal(t, firewall.MatchPositionSrc, rule.Match[1].IP.Position)
			}

			assert.Equal(t, firewall.MatchOperationEq, rule.Match[2].Op)
			if assert.NotNil(t, rule.Match[2].IP) {
				assert.Equal(t, unknownIP, rule.Match[2].IP.Value)
				assert.Equal(t, firewall.MatchPositionDst, rule.Match[2].IP.Position)
			}

			// neq: Geneve traffic (GwExtMark) must NOT be remapped here.
			assert.Equal(t, firewall.MatchOperationNeq, rule.Match[3].Op)
			if assert.NotNil(t, rule.Match[3].Mark) {
				assert.Equal(t, fmt.Sprintf("%d", utils.GwExtMark), rule.Match[3].Mark.Value)
			}
		})
	}
}

func TestCIDRRulesNotGeneratedWithoutRemapping(t *testing.T) {
	tests := []struct {
		name     string
		cidrType CIDRType
		fill     func(cfg *networkingv1beta1.Configuration)
	}{
		{
			name:     "pod cidr, identical pairs",
			cidrType: PodCIDR,
			fill: func(cfg *networkingv1beta1.Configuration) {
				cfg.Spec.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.100.0.0/16", "10.101.0.0/16"}
				cfg.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.100.0.0/16", "10.101.0.0/16"}
			},
		},
		{
			name:     "external cidr, identical pairs",
			cidrType: ExternalCIDR,
			fill: func(cfg *networkingv1beta1.Configuration) {
				cfg.Spec.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.100.0.0/16"}
				cfg.Status.Remote.CIDR.External = []networkingv1beta1.CIDR{"10.100.0.0/16"}
			},
		},
		{
			name:     "empty lists",
			cidrType: PodCIDR,
			fill:     func(_ *networkingv1beta1.Configuration) {},
		},
		{
			name:     "unknown cidr type",
			cidrType: CIDRType("other"),
			fill: func(cfg *networkingv1beta1.Configuration) {
				// Remapped pairs exist, but the type does not select any list.
				cfg.Spec.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.200.0.0/16"}
				cfg.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.61.0.0/16"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfiguration()
			cfg.Spec.Local.CIDR.External = []networkingv1beta1.CIDR{"10.70.0.0/16"}
			tt.fill(cfg)

			opts := &Options{DefaultInterfaceName: "eth0"}
			assert.Empty(t, forgeCIDRFirewallConfigurationDNATRules(cfg, opts, tt.cidrType))
			assert.Empty(t, forgeCIDRFirewallConfigurationSNATRules(cfg, opts, tt.cidrType))
		})
	}
}

func TestCIDRSNATWithoutUnknownSourceIP(t *testing.T) {
	tests := []struct {
		name  string
		local func(cfg *networkingv1beta1.Configuration)
	}{
		{
			name:  "no local external cidr",
			local: func(_ *networkingv1beta1.Configuration) {},
		},
		{
			name: "invalid local external cidr",
			local: func(cfg *networkingv1beta1.Configuration) {
				cfg.Spec.Local.CIDR.External = []networkingv1beta1.CIDR{"not-a-cidr"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfiguration()
			tt.local(cfg)
			cfg.Spec.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.200.0.0/16", "10.100.0.0/16"}
			cfg.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.61.0.0/16", "10.100.0.0/16"}

			rules := forgeCIDRFirewallConfigurationSNATRules(cfg, &Options{DefaultInterfaceName: "eth0"}, PodCIDR)

			// Only rule 1 is generated: rule 2 is skipped without an unknown source IP.
			if !assert.Len(t, rules, 1) {
				return
			}

			// Rule 1 is the one matching GwNodeMark.
			if assert.Len(t, rules[0].Match, 3) && assert.NotNil(t, rules[0].Match[2].Mark) {
				assert.Equal(t, fmt.Sprintf("%d", utils.GwNodeMark), rules[0].Match[2].Mark.Value)
			}

			// No IP match with an empty value is left.
			for i := range rules {
				for j := range rules[i].Match {
					if ip := rules[i].Match[j].IP; ip != nil {
						assert.NotEmpty(t, ip.Value)
					}
				}
			}
		})
	}
}
