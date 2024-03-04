// Copyright 2019-2024 The Liqo Authors
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
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
)

const configurationName = "service-nodeport-routing"

func generateInternalNodeRouteConfigurationName(nodename string) string {
	return fmt.Sprintf("%s-%s", nodename, configurationName)
}

func enforceRouteWithConntrackPresence(ctx context.Context, cl client.Client,
	internalnode *networkingv1alpha1.InternalNode, scheme *runtime.Scheme, mark int, nodePortSrcIP string, opts *Options) error {
	fwcfg := &networkingv1alpha1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: configurationName, Namespace: opts.Namespace},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, cl, fwcfg,
		forgeFirewallConfigurationMutateFunction(internalnode, fwcfg, mark, nodePortSrcIP)); err != nil {
		return fmt.Errorf("an error occurred while creating or updating the firewall configuration: %w", err)
	}

	routecfg := &networkingv1alpha1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: generateInternalNodeRouteConfigurationName(internalnode.Name), Namespace: opts.Namespace},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, cl, routecfg,
		forgeRouteConfigurationMutateFunction(internalnode, routecfg, scheme, mark, nodePortSrcIP)); err != nil {
		return fmt.Errorf("an error occurred while creating or updating the route configuration: %w", err)
	}

	return nil
}

func enforceRouteWithConntrackAbsence(ctx context.Context, cl client.Client,
	internalnode *networkingv1alpha1.InternalNode, opts *Options) error {
	fwcfg := &networkingv1alpha1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: configurationName, Namespace: opts.Namespace},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, cl, fwcfg,
		cleanFirewallConfigurationMutateFunction(internalnode, fwcfg)); err != nil {
		return fmt.Errorf("an error occurred while cleaning the firewall configuration: %w", err)
	}

	// We don't need to clean routeconfigurations since they have an owner reference on the node.
	return nil
}

func forgeFirewallConfigurationMutateFunction(internalnode *networkingv1alpha1.InternalNode,
	fwcfg *networkingv1alpha1.FirewallConfiguration, mark int, nodePortSrcIP string) controllerutil.MutateFn {
	return func() error {
		fwcfg.SetLabels(gateway.ForgeFirewallInternalTargetLabels())
		fwcfg.Spec.Table.Name = ptr.To(configurationName)
		fwcfg.Spec.Table.Family = ptr.To(firewall.TableFamilyINet)
		enforceFirewallConfigurationForwardChain(fwcfg, internalnode, mark, nodePortSrcIP)
		enforceFirewallConfigurationPreroutingChain(fwcfg, nodePortSrcIP)
		return nil
	}
}

func enforceFirewallConfigurationForwardChain(fwcfg *networkingv1alpha1.FirewallConfiguration,
	internalnode *networkingv1alpha1.InternalNode, mark int, nodePortSrcIP string) {
	if len(fwcfg.Spec.Table.Chains) == 0 {
		fwcfg.Spec.Table.Chains = append(fwcfg.Spec.Table.Chains, firewall.Chain{})
	}
	fwcfg.Spec.Table.Chains[0].Name = ptr.To("mark-to-conntrack")
	fwcfg.Spec.Table.Chains[0].Type = ptr.To(firewall.ChainTypeFilter)
	fwcfg.Spec.Table.Chains[0].Policy = ptr.To(firewall.ChainPolicyAccept)
	fwcfg.Spec.Table.Chains[0].Hook = &firewall.ChainHookForward
	fwcfg.Spec.Table.Chains[0].Priority = &firewall.ChainPriorityFilter
	enforceFirewallConfigurationForwardChainRules(fwcfg, internalnode, mark, nodePortSrcIP)
}

func enforceFirewallConfigurationForwardChainRules(fwcfg *networkingv1alpha1.FirewallConfiguration,
	internalnode *networkingv1alpha1.InternalNode, mark int, nodePortSrcIP string) {
	rules := &fwcfg.Spec.Table.Chains[0].Rules
	rule := forgeFirewallConfigurationForwardChainRule(internalnode, mark, nodePortSrcIP)
	if !existsFirewallConfigurationChainRule(rules.FilterRules, &rule) {
		rules.FilterRules = append(rules.FilterRules, rule)
	}
}

func forgeFirewallConfigurationForwardChainRule(internalnode *networkingv1alpha1.InternalNode, mark int, nodePortSrcIP string) firewall.FilterRule {
	return firewall.FilterRule{
		Name:   &internalnode.Name,
		Action: firewall.ActionCtMark,
		Value:  ptr.To(fmt.Sprintf("%d", mark)),
		Match: []firewall.Match{
			{
				Op: firewall.MatchOperationEq,
				IP: &firewall.MatchIP{
					Position: firewall.MatchIPPositionSrc,
					Value:    nodePortSrcIP,
				},
			},
			{
				Op: firewall.MatchOperationEq,
				Dev: &firewall.MatchDev{
					Position: firewall.MatchDevPositionIn,
					Value:    internalnode.Spec.Interface.Gateway.Name,
				},
			},
		},
	}
}

func enforceFirewallConfigurationPreroutingChain(fwcfg *networkingv1alpha1.FirewallConfiguration, nodePortSrcIP string) {
	if len(fwcfg.Spec.Table.Chains) == 0 {
		fwcfg.Spec.Table.Chains = append(fwcfg.Spec.Table.Chains, firewall.Chain{})
	}
	if len(fwcfg.Spec.Table.Chains) == 1 {
		fwcfg.Spec.Table.Chains = append(fwcfg.Spec.Table.Chains, firewall.Chain{})
	}
	fwcfg.Spec.Table.Chains[1].Name = ptr.To("conntrack-mark-to-meta-mark")
	fwcfg.Spec.Table.Chains[1].Type = ptr.To(firewall.ChainTypeFilter)
	fwcfg.Spec.Table.Chains[1].Policy = ptr.To(firewall.ChainPolicyAccept)
	fwcfg.Spec.Table.Chains[1].Hook = ptr.To(firewall.ChainHookPrerouting)
	fwcfg.Spec.Table.Chains[1].Priority = ptr.To(firewall.ChainPriorityFilter)
	fwcfg.Spec.Table.Chains[1].Rules.FilterRules = []firewall.FilterRule{
		forgeFirewallConfigurationPreroutingChainRule(nodePortSrcIP),
	}
}

func forgeFirewallConfigurationPreroutingChainRule(nodePortSrcIP string) firewall.FilterRule {
	return firewall.FilterRule{
		Name:   ptr.To("conntrack-mark-to-meta-mark "),
		Action: firewall.ActionSetMetaMarkFromCtMark,
		Match: []firewall.Match{
			{
				Op: firewall.MatchOperationEq,
				IP: &firewall.MatchIP{
					Position: firewall.MatchIPPositionDst,
					Value:    nodePortSrcIP,
				},
			},
			{
				Op: firewall.MatchOperationEq,
				Dev: &firewall.MatchDev{
					Position: firewall.MatchDevPositionIn,
					Value:    tunnel.TunnelInterfaceName,
				},
			},
		},
	}
}

func existsFirewallConfigurationChainRule(rules []firewall.FilterRule, rule *firewall.FilterRule) bool {
	for _, r := range rules {
		if *r.Name == *rule.Name {
			return true
		}
	}
	return false
}

func forgeRouteConfigurationMutateFunction(internalnode *networkingv1alpha1.InternalNode,
	routecfg *networkingv1alpha1.RouteConfiguration, scheme *runtime.Scheme, mark int, nodePortSrcIP string) controllerutil.MutateFn {
	return func() error {
		routecfg.SetLabels(gateway.ForgeRouteInternalTargetLabels())
		if err := controllerutil.SetOwnerReference(internalnode, routecfg, scheme); err != nil {
			return err
		}
		routecfg.Spec.Table.Name = generateInternalNodeRouteConfigurationName(internalnode.Name)
		enforceRouteConfigurationRule(routecfg, internalnode, mark, nodePortSrcIP)
		return nil
	}
}

func enforceRouteConfigurationRule(routecfg *networkingv1alpha1.RouteConfiguration,
	internalnode *networkingv1alpha1.InternalNode, mark int, nodePortSrcIP string) {
	routecfg.Spec.Table.Rules = []networkingv1alpha1.Rule{forgeRouteConfigurationRule(internalnode, mark, nodePortSrcIP)}
}

func forgeRouteConfigurationRule(internalnode *networkingv1alpha1.InternalNode, mark int, nodePortSrcIP string) networkingv1alpha1.Rule {
	return networkingv1alpha1.Rule{
		FwMark: &mark,
		Dst:    ptr.To(networkingv1alpha1.CIDR(fmt.Sprintf("%s/32", nodePortSrcIP))),
		Routes: []networkingv1alpha1.Route{
			{
				Dst: ptr.To(networkingv1alpha1.CIDR(fmt.Sprintf("%s/32", nodePortSrcIP))),
				Dev: ptr.To(internalnode.Spec.Interface.Gateway.Name),
				Gw:  ptr.To(internalnode.Spec.Interface.Node.IP),
			},
		},
		TargetRef: &corev1.ObjectReference{
			Name: internalnode.Name,
			Kind: networkingv1alpha1.InternalNodeKind,
		},
	}
}

func cleanFirewallConfigurationMutateFunction(internalnode *networkingv1alpha1.InternalNode,
	fwcfg *networkingv1alpha1.FirewallConfiguration) controllerutil.MutateFn {
	return func() error {
		cleanFirewallConfigurationChains(fwcfg, internalnode)
		return nil
	}
}

func cleanFirewallConfigurationChains(fwcfg *networkingv1alpha1.FirewallConfiguration,
	internalnode *networkingv1alpha1.InternalNode) {
	for i := range fwcfg.Spec.Table.Chains {
		cleanFirewallConfigurationChain(&fwcfg.Spec.Table.Chains[i], internalnode)
	}
}

func cleanFirewallConfigurationChain(chain *firewall.Chain,
	internalnode *networkingv1alpha1.InternalNode) {
	chain.Rules.FilterRules = slices.DeleteFunc(
		chain.Rules.FilterRules, func(r firewall.FilterRule) bool {
			return *r.Name == internalnode.Name
		})
}
