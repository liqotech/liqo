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

package route

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

const (
	configurationNameSvc     = "service-nodeport-routing"
	configurationNameExtCIDR = "extcidr"
)

func generateInternalNodeSvcRouteConfigurationName(nodename string) string {
	return fmt.Sprintf("%s-%s", nodename, configurationNameSvc)
}

func generateInternalNodeExtCIDRRouteConfigurationName(nodename string) string {
	return fmt.Sprintf("%s-%s", nodename, configurationNameExtCIDR)
}

func enforceRouteWithConntrackPresence(ctx context.Context, cl client.Client,
	internalnode *networkingv1beta1.InternalNode, scheme *runtime.Scheme, mark int, nodePortSrcIP string, opts *Options) error {
	fwcfg := &networkingv1beta1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: configurationNameSvc, Namespace: opts.Namespace},
	}

	if _, err := resource.CreateOrUpdate(ctx, cl, fwcfg,
		forgeFirewallConfigurationMutateFunction(internalnode, fwcfg, mark, nodePortSrcIP)); err != nil {
		return fmt.Errorf("an error occurred while creating or updating the firewall configuration: %w", err)
	}

	routecfg := &networkingv1beta1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: generateInternalNodeSvcRouteConfigurationName(internalnode.Name), Namespace: opts.Namespace},
	}

	if _, err := resource.CreateOrUpdate(ctx, cl, routecfg,
		forgeRouteConfigurationMutateFunction(internalnode, routecfg, scheme, mark, nodePortSrcIP)); err != nil {
		return fmt.Errorf("an error occurred while creating or updating the route configuration: %w", err)
	}

	return nil
}

func enforceRouteWithConntrackAbsence(ctx context.Context, cl client.Client,
	internalnode *networkingv1beta1.InternalNode, opts *Options) error {
	fwcfg := &networkingv1beta1.FirewallConfiguration{}

	err := cl.Get(ctx, client.ObjectKey{Name: configurationNameSvc, Namespace: opts.Namespace}, fwcfg)
	if k8serrors.IsNotFound(err) {
		// If the firewall configuration does not exist no needs to clean things up.
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to get firewall configuration: %w", err)
	}

	// We need to remove from the firewall configurations all the rules related to the InternalNode to be remove
	cleanFirewallConfigurationChains(fwcfg, internalnode)
	if err := cl.Update(ctx, fwcfg); err != nil {
		return fmt.Errorf("an error occurred while cleaning the firewall configuration: %w", err)
	}

	// If there are no firewall configurations left, delete the resource
	if err := deleteVoidFwcfg(ctx, cl, fwcfg); err != nil {
		return fmt.Errorf("an error occurred while deleting the firewall configuration: %w", err)
	}

	// We don't need to clean routeconfigurations since they have an owner reference on the node.
	return nil
}

func deleteVoidFwcfg(ctx context.Context, cl client.Client, fwcfg *networkingv1beta1.FirewallConfiguration) error {
	if len(fwcfg.Spec.Table.Chains) > 0 && len(fwcfg.Spec.Table.Chains[0].Rules.FilterRules) == 0 {
		if err := cl.Delete(ctx, fwcfg); err != nil {
			return fmt.Errorf("an error occurred while deleting the firewall configuration: %w", err)
		}
	}
	return nil
}

func forgeFirewallConfigurationMutateFunction(internalnode *networkingv1beta1.InternalNode,
	fwcfg *networkingv1beta1.FirewallConfiguration, mark int, nodePortSrcIP string) controllerutil.MutateFn {
	return func() error {
		fwcfg.SetLabels(gateway.ForgeFirewallInternalTargetLabels())
		fwcfg.Spec.Table.Name = ptr.To(configurationNameSvc)
		fwcfg.Spec.Table.Family = ptr.To(firewall.TableFamilyIPv4)
		enforceFirewallConfigurationForwardChain(fwcfg, internalnode, mark, nodePortSrcIP)
		enforceFirewallConfigurationPreroutingChain(fwcfg, nodePortSrcIP)
		return nil
	}
}

func enforceFirewallConfigurationForwardChain(fwcfg *networkingv1beta1.FirewallConfiguration,
	internalnode *networkingv1beta1.InternalNode, mark int, nodePortSrcIP string) {
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

func enforceFirewallConfigurationForwardChainRules(fwcfg *networkingv1beta1.FirewallConfiguration,
	internalnode *networkingv1beta1.InternalNode, mark int, nodePortSrcIP string) {
	rules := &fwcfg.Spec.Table.Chains[0].Rules
	rule := forgeFirewallConfigurationForwardChainRule(internalnode, mark, nodePortSrcIP)
	if !existsFirewallConfigurationChainRule(rules.FilterRules, &rule) {
		rules.FilterRules = append(rules.FilterRules, rule)
	}
}

func forgeFirewallConfigurationForwardChainRule(internalnode *networkingv1beta1.InternalNode, mark int, nodePortSrcIP string) firewall.FilterRule {
	return firewall.FilterRule{
		Name:   &internalnode.Name,
		Action: firewall.ActionCtMark,
		Value:  ptr.To(fmt.Sprintf("%d", mark)),
		Match: []firewall.Match{
			{
				Op: firewall.MatchOperationEq,
				IP: &firewall.MatchIP{
					Position: firewall.MatchPositionSrc,
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

func enforceFirewallConfigurationPreroutingChain(fwcfg *networkingv1beta1.FirewallConfiguration, nodePortSrcIP string) {
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
		Name:   ptr.To("conntrack-mark-to-meta-mark"),
		Action: firewall.ActionSetMetaMarkFromCtMark,
		Match: []firewall.Match{
			{
				Op: firewall.MatchOperationEq,
				IP: &firewall.MatchIP{
					Position: firewall.MatchPositionDst,
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

func forgeRouteConfigurationMutateFunction(internalnode *networkingv1beta1.InternalNode,
	routecfg *networkingv1beta1.RouteConfiguration, scheme *runtime.Scheme, mark int, nodePortSrcIP string) controllerutil.MutateFn {
	return func() error {
		routecfg.SetLabels(gateway.ForgeRouteInternalTargetLabels())
		if err := controllerutil.SetOwnerReference(internalnode, routecfg, scheme); err != nil {
			return err
		}
		routecfg.Spec.Table.Name = generateInternalNodeSvcRouteConfigurationName(internalnode.Name)
		enforceRouteConfigurationRules(routecfg, internalnode, mark, nodePortSrcIP)
		return nil
	}
}

func enforceRouteConfigurationRules(routecfg *networkingv1beta1.RouteConfiguration,
	internalnode *networkingv1beta1.InternalNode, mark int, nodePortSrcIP string) {
	routecfg.Spec.Table.Rules = forgeRouteConfigurationRules(internalnode, mark, nodePortSrcIP)
}

func forgeRouteConfigurationRules(internalnode *networkingv1beta1.InternalNode, mark int, nodePortSrcIP string) []networkingv1beta1.Rule {
	return []networkingv1beta1.Rule{
		{
			FwMark: &mark,
			Dst:    ptr.To(networkingv1beta1.CIDR(fmt.Sprintf("%s/32", nodePortSrcIP))),
			Routes: []networkingv1beta1.Route{
				{
					Dst: ptr.To(networkingv1beta1.CIDR(fmt.Sprintf("%s/32", nodePortSrcIP))),
					Dev: ptr.To(internalnode.Spec.Interface.Gateway.Name),
					Gw:  ptr.To(internalnode.Spec.Interface.Node.IP),
				},
			},
			TargetRef: &corev1.ObjectReference{
				Name: internalnode.Name,
				Kind: networkingv1beta1.InternalNodeKind,
			},
		},
	}
}

func cleanFirewallConfigurationChains(fwcfg *networkingv1beta1.FirewallConfiguration,
	internalnode *networkingv1beta1.InternalNode) {
	for i := range fwcfg.Spec.Table.Chains {
		cleanFirewallConfigurationChain(&fwcfg.Spec.Table.Chains[i], internalnode)
	}
}

func cleanFirewallConfigurationChain(chain *firewall.Chain,
	internalnode *networkingv1beta1.InternalNode) {
	chain.Rules.FilterRules = slices.DeleteFunc(
		chain.Rules.FilterRules, func(r firewall.FilterRule) bool {
			return *r.Name == internalnode.Name
		})
}

func enforceRouteConfigurationExtCIDR(ctx context.Context, cl client.Client,
	internalnode *networkingv1beta1.InternalNode, configurations []networkingv1beta1.Configuration,
	ips []ipamv1alpha1.IP, scheme *runtime.Scheme, opts *Options) error {
	routecfg := &networkingv1beta1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: generateInternalNodeExtCIDRRouteConfigurationName(internalnode.Name), Namespace: opts.Namespace},
	}

	if _, err := resource.CreateOrUpdate(ctx, cl, routecfg,
		forgeRouteConfigurationExtCIDRMutateFunction(internalnode, routecfg, configurations, ips, scheme)); err != nil {
		return fmt.Errorf("an error occurred while creating or updating the route configuration: %w", err)
	}
	return nil
}

func forgeRouteConfigurationExtCIDRMutateFunction(internalnode *networkingv1beta1.InternalNode,
	routecfg *networkingv1beta1.RouteConfiguration, configurations []networkingv1beta1.Configuration,
	ips []ipamv1alpha1.IP, scheme *runtime.Scheme) controllerutil.MutateFn {
	return func() error {
		routecfg.SetLabels(gateway.ForgeRouteInternalTargetLabelsByNode(internalnode.Name))
		if err := controllerutil.SetOwnerReference(internalnode, routecfg, scheme); err != nil {
			return err
		}
		routecfg.Spec.Table.Name = generateInternalNodeExtCIDRRouteConfigurationName(internalnode.Name)
		routecfg.Spec.Table.Rules = forgeRouteConfigurationExtCIDRRules(internalnode, configurations, ips)
		return nil
	}
}

func forgeRouteConfigurationExtCIDRRules(internalnode *networkingv1beta1.InternalNode,
	configurations []networkingv1beta1.Configuration, ips []ipamv1alpha1.IP) []networkingv1beta1.Rule {
	rules := []networkingv1beta1.Rule{}
	for i := range configurations {
		rules = append(rules, networkingv1beta1.Rule{
			Dst:    cidrutils.GetPrimary(configurations[i].Status.Remote.CIDR.Pod),
			Iif:    ptr.To(tunnel.TunnelInterfaceName),
			Routes: forgeRouteConfigurationExtCIDRRoutes(internalnode, cidrutils.GetPrimary(configurations[i].Status.Remote.CIDR.Pod)),
		})
	}
	rules = append(rules, networkingv1beta1.Rule{
		Iif:    ptr.To(tunnel.TunnelInterfaceName),
		Routes: forgeRouteConfigurationExtCIDRRoutesIP(internalnode, ips),
	})
	return rules
}

func forgeRouteConfigurationExtCIDRRoutes(internalnode *networkingv1beta1.InternalNode, dst *networkingv1beta1.CIDR) []networkingv1beta1.Route {
	return []networkingv1beta1.Route{
		{
			Dst: dst,
			Dev: ptr.To(internalnode.Spec.Interface.Gateway.Name),
			Gw:  ptr.To(internalnode.Spec.Interface.Node.IP),
		},
	}
}
func forgeRouteConfigurationExtCIDRRoutesIP(internalnode *networkingv1beta1.InternalNode, ips []ipamv1alpha1.IP) []networkingv1beta1.Route {
	routes := []networkingv1beta1.Route{}
	for i := range ips {
		routes = append(routes, networkingv1beta1.Route{
			Dst: ptr.To(networkingv1beta1.CIDR(fmt.Sprintf("%s/32", ips[i].Spec.IP.String()))),
			Dev: ptr.To(internalnode.Spec.Interface.Gateway.Name),
			Gw:  ptr.To(internalnode.Spec.Interface.Node.IP),
		})
	}
	return routes
}
