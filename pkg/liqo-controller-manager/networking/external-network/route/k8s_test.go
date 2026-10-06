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

package route

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/remapping"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/utils"
	internalnetwork "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, networkingv1beta1.AddToScheme(scheme))
	return scheme
}

func newTestConfiguration() *networkingv1beta1.Configuration {
	cfg := &networkingv1beta1.Configuration{}
	cfg.Name = "rome"
	cfg.Namespace = "liqo-tenant-rome"
	cfg.UID = "cfg-uid"
	return cfg
}

func TestForgeMutateFirewallConfigurationMarks(t *testing.T) {
	tests := []struct {
		name      string
		chainName string
		prefix    string
		mark      uint32
	}{
		{"gw-ext-mark", "gw-ext-mark", internalnetwork.InterfaceNamePrefix, utils.GwExtMark},
		{"gw-node-mark", "gw-node-mark", tunnel.TunnelInterfaceName, utils.GwNodeMark},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfiguration()
			fwcfg := &networkingv1beta1.FirewallConfiguration{}
			fwcfg.Namespace = cfg.Namespace
			remoteID := liqov1beta1.ClusterID("rome-id")

			err := forgeMutateFirewallConfiguration(cfg, fwcfg, newTestScheme(t), remoteID,
				tt.chainName, tt.prefix, tt.mark)()
			require.NoError(t, err)

			// Labels and owner.
			assert.Equal(t, remapping.ForgeFirewallTargetLabels(string(remoteID)), fwcfg.Labels)
			assert.Len(t, fwcfg.OwnerReferences, 1)

			// Table.
			table := fwcfg.Spec.Table
			require.NotNil(t, table.Name)
			assert.Equal(t, "rome-"+tt.chainName, *table.Name)
			require.NotNil(t, table.Family)
			assert.Equal(t, firewall.TableFamilyIPv4, *table.Family)

			// Chain.
			require.Len(t, table.Chains, 1)
			chain := table.Chains[0]
			require.NotNil(t, chain.Name)
			assert.Equal(t, "rome-"+tt.chainName, *chain.Name)
			assert.Equal(t, firewall.ChainTypeFilter, chain.Type)
			require.NotNil(t, chain.Hook)
			assert.Equal(t, firewall.ChainHookPrerouting, *chain.Hook)
			require.NotNil(t, chain.Priority)
			assert.Equal(t, firewall.ChainPriorityMangle, *chain.Priority)
			require.NotNil(t, chain.Policy)
			assert.Equal(t, firewall.ChainPolicyAccept, *chain.Policy)

			// Rule.
			require.Len(t, chain.Rules.FilterRules, 1)
			rule := chain.Rules.FilterRules[0]
			require.NotNil(t, rule.Name)
			assert.Equal(t, tt.chainName, *rule.Name)
			assert.Equal(t, firewall.ActionSetMetaMark, rule.Action)
			require.NotNil(t, rule.Value)
			assert.Equal(t, fmt.Sprintf("%d", tt.mark), *rule.Value)

			// Match: wildcard dev in, prefix only.
			require.Len(t, rule.Match, 1)
			m := rule.Match[0]
			assert.Equal(t, firewall.MatchOperationEq, m.Op)
			require.NotNil(t, m.Dev)
			assert.Equal(t, tt.prefix, m.Dev.Value)
			assert.Equal(t, firewall.MatchDevPositionIn, m.Dev.Position)
			assert.True(t, m.Dev.Wildcard)
		})
	}
}

func TestForgeMutateRouteConfigurationUsesGwExtMark(t *testing.T) {
	tests := []struct {
		name     string
		pod      []networkingv1beta1.CIDR
		external []networkingv1beta1.CIDR
		expected []networkingv1beta1.CIDR
	}{
		{"pod and external", []networkingv1beta1.CIDR{"10.200.0.0/16"},
			[]networkingv1beta1.CIDR{"10.61.0.0/16"},
			[]networkingv1beta1.CIDR{"10.200.0.0/16", "10.61.0.0/16"}},
		{"only pod", []networkingv1beta1.CIDR{"10.200.0.0/16", "10.201.0.0/16"}, nil,
			[]networkingv1beta1.CIDR{"10.200.0.0/16", "10.201.0.0/16"}},
		{"only external", nil, []networkingv1beta1.CIDR{"10.61.0.0/16"},
			[]networkingv1beta1.CIDR{"10.61.0.0/16"}},
		{"no cidrs", nil, nil, nil},
		{
			name: "many cidrs",
			pod: []networkingv1beta1.CIDR{
				"10.200.0.0/16", "10.201.0.0/16", "10.202.0.0/16", "10.203.0.0/16", "10.204.0.0/16",
			},
			external: []networkingv1beta1.CIDR{
				"10.61.0.0/16", "10.62.0.0/16", "10.63.0.0/16", "10.64.0.0/16", "10.65.0.0/16",
			},
			expected: []networkingv1beta1.CIDR{
				"10.200.0.0/16", "10.201.0.0/16", "10.202.0.0/16", "10.203.0.0/16", "10.204.0.0/16",
				"10.61.0.0/16", "10.62.0.0/16", "10.63.0.0/16", "10.64.0.0/16", "10.65.0.0/16",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfiguration()
			cfg.Spec.Remote.CIDR.Pod = tt.pod
			cfg.Spec.Remote.CIDR.External = tt.external
			routecfg := &networkingv1beta1.RouteConfiguration{}
			routecfg.Namespace = cfg.Namespace
			remoteID := liqov1beta1.ClusterID("rome-id")

			err := forgeMutateRouteConfiguration(cfg, routecfg, newTestScheme(t), remoteID, "10.0.0.2")()
			require.NoError(t, err)

			assert.Len(t, routecfg.OwnerReferences, 1)
			assert.Equal(t, cfg.Name, routecfg.Spec.Table.Name)

			rules := routecfg.Spec.Table.Rules
			require.Len(t, rules, len(tt.expected))
			for i, rule := range rules {
				require.NotNil(t, rule.FwMark)
				assert.Equal(t, int(utils.GwExtMark), *rule.FwMark)
				assert.Nil(t, rule.Iif)
				require.NotNil(t, rule.Dst)
				assert.Equal(t, tt.expected[i], *rule.Dst)
				require.Len(t, rule.Routes, 1)
				require.NotNil(t, rule.Routes[0].Dst)
				assert.Equal(t, tt.expected[i], *rule.Routes[0].Dst)
				require.NotNil(t, rule.Routes[0].Gw)
				assert.Equal(t, networkingv1beta1.IP("10.0.0.2"), *rule.Routes[0].Gw)
			}
		})
	}
}

func TestGenerateNames(t *testing.T) {
	cfg := newTestConfiguration() // name: "rome"

	assert.Equal(t, "rome-gw-ext", GenerateRouteConfigurationName(cfg))

	tests := []struct {
		suffix   string
		expected string
	}{
		{"gw-ext", "rome-gw-ext"},
		{"gw-node", "rome-gw-node"},
	}
	for _, tt := range tests {
		t.Run(tt.suffix, func(t *testing.T) {
			assert.Equal(t, tt.expected, GenerateFirewallConfigurationName(cfg, tt.suffix))
		})
	}

	// The two firewall configurations must never collide.
	assert.NotEqual(t,
		GenerateFirewallConfigurationName(cfg, "gw-ext"),
		GenerateFirewallConfigurationName(cfg, "gw-node"))
}
