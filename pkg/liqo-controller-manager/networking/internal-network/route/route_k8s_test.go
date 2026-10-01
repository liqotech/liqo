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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	utils "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/utils"
)

// Helpers.

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, networkingv1beta1.AddToScheme(scheme))
	return scheme
}

func newTestInternalNode() *networkingv1beta1.InternalNode {
	node := &networkingv1beta1.InternalNode{}
	node.Name = "worker-1"
	node.UID = "node-uid"
	node.Spec.Interface.Node.IP = "10.0.0.1"
	node.Spec.Interface.Gateway.Name = "gw-iface"
	return node
}

func newTestPod(hostNetwork bool) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = "pod-a"
	pod.Namespace = "default"
	pod.UID = "pod-uid"
	pod.Spec.NodeName = "worker-1"
	pod.Spec.HostNetwork = hostNetwork
	pod.Status.PodIP = "10.1.0.5"
	return pod
}

func newTestRouteConfiguration() *networkingv1beta1.RouteConfiguration {
	routecfg := &networkingv1beta1.RouteConfiguration{}
	routecfg.Namespace = "liqo"
	return routecfg
}

// pod_k8s.go

func TestForgeRoutePodUpdateUsesGwNodeMark(t *testing.T) {
	node := newTestInternalNode()
	pod := newTestPod(false)
	routecfg := newTestRouteConfiguration()

	err := forgeRoutePodUpdateFunction(node, routecfg, pod, newTestScheme(t))()
	require.NoError(t, err)

	assert.Equal(t, "worker-1", routecfg.Spec.Table.Name)
	require.Len(t, routecfg.Spec.Table.Rules, 2)

	// Rule 0: route to the node itself.
	nodeRule := routecfg.Spec.Table.Rules[0]
	require.NotNil(t, nodeRule.Dst)
	assert.Equal(t, networkingv1beta1.CIDR("10.0.0.1/32"), *nodeRule.Dst)
	require.Len(t, nodeRule.Routes, 1)
	require.NotNil(t, nodeRule.Routes[0].Dev)
	assert.Equal(t, "gw-iface", *nodeRule.Routes[0].Dev)

	// Rule 1: pods, selected by mark and not by interface.
	podRule := routecfg.Spec.Table.Rules[1]
	require.NotNil(t, podRule.FwMark)
	assert.Equal(t, int(utils.GwNodeMark), *podRule.FwMark)
	assert.Nil(t, podRule.Iif)

	require.Len(t, podRule.Routes, 1)
	route := podRule.Routes[0]
	require.NotNil(t, route.Dst)
	assert.Equal(t, networkingv1beta1.CIDR("10.1.0.5/32"), *route.Dst)
	require.NotNil(t, route.Gw)
	assert.Equal(t, node.Spec.Interface.Node.IP, *route.Gw)
	require.NotNil(t, route.TargetRef)
	assert.Equal(t, "pod-a", route.TargetRef.Name)
	assert.Equal(t, "default", route.TargetRef.Namespace)
}

func TestForgeRoutePodUpdateHostNetworkHasNoPodRule(t *testing.T) {
	node := newTestInternalNode()
	pod := newTestPod(true)
	routecfg := newTestRouteConfiguration()

	err := forgeRoutePodUpdateFunction(node, routecfg, pod, newTestScheme(t))()
	require.NoError(t, err)

	// Only the rule for the node itself: no pod rule, hence no mark rule.
	require.Len(t, routecfg.Spec.Table.Rules, 1)
	nodeRule := routecfg.Spec.Table.Rules[0]
	require.NotNil(t, nodeRule.Dst)
	assert.Equal(t, networkingv1beta1.CIDR("10.0.0.1/32"), *nodeRule.Dst)
	assert.Nil(t, nodeRule.FwMark)
}

// internalnode_k8s.go

func TestForgeFirewallConfigurationPreroutingChainRuleUsesGwNodeMark(t *testing.T) {
	rule := forgeFirewallConfigurationPreroutingChainRule("10.70.0.0")

	require.NotNil(t, rule.Name)
	assert.Equal(t, "conntrack-mark-to-meta-mark", *rule.Name)
	assert.Equal(t, firewall.ActionSetMetaMarkFromCtMark, rule.Action)

	require.Len(t, rule.Match, 2)

	// Destination is the NodePort source IP.
	assert.Equal(t, firewall.MatchOperationEq, rule.Match[0].Op)
	require.NotNil(t, rule.Match[0].IP)
	assert.Equal(t, "10.70.0.0", rule.Match[0].IP.Value)
	assert.Equal(t, firewall.MatchPositionDst, rule.Match[0].IP.Position)

	// Traffic coming from WireGuard is identified by the mark, not by the interface.
	assert.Equal(t, firewall.MatchOperationEq, rule.Match[1].Op)
	require.NotNil(t, rule.Match[1].Mark)
	assert.Equal(t, fmt.Sprintf("%d", utils.GwNodeMark), rule.Match[1].Mark.Value)

	for i := range rule.Match {
		assert.Nil(t, rule.Match[i].Dev)
	}
}

func TestForgeRouteConfigurationExtCIDRRulesUseGwNodeMark(t *testing.T) {
	node := newTestInternalNode()

	cfg1 := networkingv1beta1.Configuration{}
	cfg1.Status.Remote = &networkingv1beta1.ClusterConfig{}
	cfg1.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.61.0.0/16", "10.62.0.0/16"}
	cfg2 := networkingv1beta1.Configuration{}
	cfg2.Status.Remote = &networkingv1beta1.ClusterConfig{}
	cfg2.Status.Remote.CIDR.Pod = []networkingv1beta1.CIDR{"10.63.0.0/16"}

	ip := ipamv1alpha1.IP{}
	ip.Spec.IP = "10.70.0.5"

	rules := forgeRouteConfigurationExtCIDRRules(node,
		[]networkingv1beta1.Configuration{cfg1, cfg2}, []ipamv1alpha1.IP{ip})

	// One rule per remote pod CIDR, plus the final rule for the IPs.
	require.Len(t, rules, 4)

	expectedDst := []networkingv1beta1.CIDR{"10.61.0.0/16", "10.62.0.0/16", "10.63.0.0/16"}
	for i, dst := range expectedDst {
		rule := rules[i]
		require.NotNil(t, rule.FwMark)
		assert.Equal(t, int(utils.GwNodeMark), *rule.FwMark)
		assert.Nil(t, rule.Iif)
		require.NotNil(t, rule.Dst)
		assert.Equal(t, dst, *rule.Dst)
		require.Len(t, rule.Routes, 1)
		assert.Equal(t, dst, *rule.Routes[0].Dst)
		assert.Equal(t, "gw-iface", *rule.Routes[0].Dev)
		assert.Equal(t, node.Spec.Interface.Node.IP, *rule.Routes[0].Gw)
	}

	// Final rule: no Dst, mark only, one /32 route per IP.
	last := rules[3]
	require.NotNil(t, last.FwMark)
	assert.Equal(t, int(utils.GwNodeMark), *last.FwMark)
	assert.Nil(t, last.Iif)
	assert.Nil(t, last.Dst)
	require.Len(t, last.Routes, 1)
	assert.Equal(t, networkingv1beta1.CIDR("10.70.0.5/32"), *last.Routes[0].Dst)
}

func TestForgeRouteConfigurationExtCIDRRulesWithoutConfigurations(t *testing.T) {
	rules := forgeRouteConfigurationExtCIDRRules(newTestInternalNode(), nil, nil)

	// Only the final rule is present, with the mark and no routes.
	require.Len(t, rules, 1)
	require.NotNil(t, rules[0].FwMark)
	assert.Equal(t, int(utils.GwNodeMark), *rules[0].FwMark)
	assert.Nil(t, rules[0].Iif)
	assert.Empty(t, rules[0].Routes)
}
