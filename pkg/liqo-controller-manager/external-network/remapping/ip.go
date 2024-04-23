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

package remapping

import (
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
)

// CreateOrUpdateNatMappingIP creates or updates the NAT mapping for an IP.
func CreateOrUpdateNatMappingIP(ctx context.Context, cl client.Client, ip *ipamv1alpha1.IP) error {
	fwcfg := &networkingv1alpha1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TableIPMappingGwName,
			Namespace: ip.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(
		ctx, cl, fwcfg,
		mutateFirewallConfiguration(fwcfg, ip),
	)

	if ip.Spec.Masquerade != nil && *ip.Spec.Masquerade {
		fwcfgMasq := &networkingv1alpha1.FirewallConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      TableIPMappingFabricName,
				Namespace: ip.Namespace,
			},
		}
		_, err = controllerutil.CreateOrUpdate(
			ctx, cl, fwcfgMasq,
			mutateFirewallConfigurationMasquerade(fwcfgMasq, ip),
		)
	}

	return err
}

// DeleteNatMappingIP deletes the NAT mapping for an IP.
func DeleteNatMappingIP(ctx context.Context, cl client.Client, ip *ipamv1alpha1.IP) error {
	var fwcfg networkingv1alpha1.FirewallConfiguration
	err := cl.Get(ctx, client.ObjectKey{Name: TableIPMappingGwName, Namespace: ip.Namespace}, &fwcfg)
	switch {
	case errors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("unable to get the firewall configuration %s/%s: %w", ip.Namespace, TableIPMappingGwName, err)
	}

	err = cl.Update(ctx, cleanFirewallConfiguration(&fwcfg, ip))
	switch {
	case errors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("unable to update the firewall configuration %q: %w", fwcfg.Name, err)
	}

	return deleteFirewallConfiguration(ctx, cl, &fwcfg)
}

func cleanFirewallConfiguration(fwcfg *networkingv1alpha1.FirewallConfiguration, ip *ipamv1alpha1.IP) *networkingv1alpha1.FirewallConfiguration {
	for i := range fwcfg.Spec.Table.Chains {
		chain := &fwcfg.Spec.Table.Chains[i]
		chain.Rules.NatRules = slices.DeleteFunc(chain.Rules.NatRules, func(r firewall.NatRule) bool {
			return r.Name != nil && *r.Name == ip.Name
		})
	}
	return fwcfg
}

func deleteFirewallConfiguration(ctx context.Context, cl client.Client, fwcfg *networkingv1alpha1.FirewallConfiguration) error {
	allChainsVoid := true
	for i := range fwcfg.Spec.Table.Chains {
		chain := &fwcfg.Spec.Table.Chains[i]
		if len(chain.Rules.NatRules) > 0 {
			allChainsVoid = false
		}
	}

	if allChainsVoid {
		if err := cl.Delete(ctx, fwcfg); err != nil {
			return err
		}
	}
	return nil
}

func mutateFirewallConfiguration(fwcfg *networkingv1alpha1.FirewallConfiguration, ip *ipamv1alpha1.IP) func() error {
	return func() error {
		fwcfg.SetLabels(ForgeFirewallTargetLabelsIPMappingGw())
		enforceFirewallConfigurationSpec(fwcfg, ip)
		return nil
	}
}

func mutateFirewallConfigurationMasquerade(fwcfg *networkingv1alpha1.FirewallConfiguration, ip *ipamv1alpha1.IP) func() error {
	return func() error {
		fwcfg.SetLabels(ForgeFirewallTargetLabelsIPMappingFabric())
		enforceFirewallConfigurationMasqSpec(fwcfg, ip)
		return nil
	}
}

func enforceFirewallConfigurationSpec(fwcfg *networkingv1alpha1.FirewallConfiguration, ip *ipamv1alpha1.IP) {
	table := &fwcfg.Spec.Table
	table.Name = ptr.To(fmt.Sprintf("%s-%s", TableIPMappingGwName, fwcfg.Namespace))
	table.Family = ptr.To(firewall.TableFamilyIPv4)
	enforceFirewallConfigurationChains(fwcfg, ip)
}

func enforceFirewallConfigurationMasqSpec(fwcfg *networkingv1alpha1.FirewallConfiguration, ip *ipamv1alpha1.IP) {
	table := &fwcfg.Spec.Table
	table.Name = ptr.To(fmt.Sprintf("%s-%s", TableIPMappingFabricName, fwcfg.Namespace))
	table.Family = ptr.To(firewall.TableFamilyIPv4)
	enforceFirewallConfigurationMasqChains(fwcfg, ip)
}

func enforceFirewallConfigurationChains(fwcfg *networkingv1alpha1.FirewallConfiguration, ip *ipamv1alpha1.IP) {
	if fwcfg.Spec.Table.Chains == nil || len(fwcfg.Spec.Table.Chains) != 2 {
		fwcfg.Spec.Table.Chains = make([]firewall.Chain, 2)
	}
	chainPre := &fwcfg.Spec.Table.Chains[0]
	chainPre.Name = &PreroutingChainName
	chainPre.Policy = ptr.To(firewall.ChainPolicyAccept)
	chainPre.Type = ptr.To(firewall.ChainTypeNAT)
	chainPre.Hook = &firewall.ChainHookPrerouting
	chainPre.Priority = ptr.To(firewall.ChainPriorityNATDest)
	ensureFirewallConfigurationDNATRules(fwcfg, ip)

	chainPost := &fwcfg.Spec.Table.Chains[1]
	chainPost.Name = &PostroutingChainName
	chainPost.Policy = ptr.To(firewall.ChainPolicyAccept)
	chainPost.Type = ptr.To(firewall.ChainTypeNAT)
	chainPost.Hook = &firewall.ChainHookPostrouting
	chainPost.Priority = ptr.To(firewall.ChainPriorityNATSource)
	ensureFirewallConfigurationSNATRules(fwcfg, ip)
}

func enforceFirewallConfigurationMasqChains(fwcfg *networkingv1alpha1.FirewallConfiguration, ip *ipamv1alpha1.IP) {
	if fwcfg.Spec.Table.Chains == nil || len(fwcfg.Spec.Table.Chains) != 1 {
		fwcfg.Spec.Table.Chains = make([]firewall.Chain, 1)
	}
	rulePost := &fwcfg.Spec.Table.Chains[0]
	rulePost.Name = &PostroutingChainName
	rulePost.Policy = ptr.To(firewall.ChainPolicyAccept)
	rulePost.Type = ptr.To(firewall.ChainTypeNAT)
	rulePost.Hook = &firewall.ChainHookPostrouting
	rulePost.Priority = ptr.To(firewall.ChainPriorityNATSource - 1)
	ensureFirewallConfigurationMasqSNATRules(fwcfg, ip)
}

func containsNATRule(rules []firewall.NatRule, to string, pos firewall.MatchIPPosition) bool {
	for i := range rules {
		if rules[i].To != nil && *rules[i].To == to {
			for j := range rules[i].Match {
				if rules[i].Match[j].IP != nil && rules[i].Match[j].IP.Position == pos {
					return true
				}
			}
		}
	}
	return false
}

// GetFirstIPFromMapping returns the first IP from the IP mapping.
func GetFirstIPFromMapping(ipMapping map[string]networkingv1alpha1.IP) string {
	for _, ip := range ipMapping {
		return ip.String()
	}
	return ""
}

func ensureFirewallConfigurationDNATRules(fwcfg *networkingv1alpha1.FirewallConfiguration,
	ip *ipamv1alpha1.IP) {
	rules := &fwcfg.Spec.Table.Chains[0].Rules
	if !containsNATRule(rules.NatRules, ip.Spec.IP.String(), firewall.MatchIPPositionDst) {
		rules.NatRules = append(rules.NatRules, firewall.NatRule{
			NatType: firewall.NatTypeDestination,
			To:      ptr.To(ip.Spec.IP.String()),
			Name:    &ip.Name,
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Position: firewall.MatchIPPositionDst,
						Value:    GetFirstIPFromMapping(ip.Status.IPMappings),
					},
				},
			},
		})
	}
}

func ensureFirewallConfigurationSNATRules(fwcfg *networkingv1alpha1.FirewallConfiguration,
	ip *ipamv1alpha1.IP) {
	rules := &fwcfg.Spec.Table.Chains[1].Rules
	if !containsNATRule(rules.NatRules, ip.Spec.IP.String(), firewall.MatchIPPositionSrc) {
		rules.NatRules = append(rules.NatRules, firewall.NatRule{
			NatType: firewall.NatTypeSource,
			To:      ptr.To(ip.Spec.IP.String()),
			Name:    &ip.Name,
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Position: firewall.MatchIPPositionSrc,
						Value:    GetFirstIPFromMapping(ip.Status.IPMappings),
					},
				},
			},
		})
	}
}

func containsNatRuleMasquerade(rules []firewall.NatRule, dst string) bool {
	for i := range rules {
		if rules[i].To == nil && rules[i].NatType == firewall.NatTypeMasquerade {
			for j := range rules[i].Match {
				if rules[i].Match[j].IP != nil && rules[i].Match[j].IP.Position == firewall.MatchIPPositionDst &&
					rules[i].Match[j].IP.Value == dst {
					return true
				}
			}
		}
	}
	return false
}

func ensureFirewallConfigurationMasqSNATRules(fwcfg *networkingv1alpha1.FirewallConfiguration,
	ip *ipamv1alpha1.IP) {
	rules := &fwcfg.Spec.Table.Chains[0].Rules
	if !containsNatRuleMasquerade(rules.NatRules, ip.Spec.IP.String()) {
		rules.NatRules = append(rules.NatRules, firewall.NatRule{
			NatType: firewall.NatTypeMasquerade,
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Position: firewall.MatchIPPositionDst,
						Value:    ip.Spec.IP.String(),
					},
				},
			},
		})
	}
}
