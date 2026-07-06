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
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

func generateNatMappingIPGwName(ip *ipamv1alpha1.IP) string {
	return fmt.Sprintf("%s-%s", ip.Name, TableIPMappingGwName)
}

func generateNatMappingIPFabricName(ip *ipamv1alpha1.IP) string {
	return fmt.Sprintf("%s-%s", ip.Name, TableIPMappingFabricName)
}

// CreateOrUpdateNatMappingIP creates or updates the NAT mapping for an IP.
func CreateOrUpdateNatMappingIP(ctx context.Context, cl client.Client, ip *ipamv1alpha1.IP) error {
	fwcfg := &networkingv1beta1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateNatMappingIPGwName(ip),
			Namespace: ip.Namespace,
		},
	}
	if err := createOrUpdateFirewallConfiguration(ctx, cl, fwcfg, mutateFirewallConfiguration(fwcfg, ip)); err != nil {
		return err
	}

	if ip.Spec.Masquerade != nil && *ip.Spec.Masquerade {
		fwcfgMasq := &networkingv1beta1.FirewallConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:      generateNatMappingIPFabricName(ip),
				Namespace: ip.Namespace,
			},
		}
		return createOrUpdateFirewallConfiguration(ctx, cl, fwcfgMasq, mutateFirewallConfigurationMasquerade(fwcfgMasq, ip))
	}

	// Masquerade is disabled (nil or false): ensure no masquerade firewall rules are left
	// over from a previous state where Masquerade was true.
	return ensureFirewallConfigurationMasqueradeAbsence(ctx, cl, ip)
}

// DeleteNatMappingIP deletes the NAT mapping for an IP.
func DeleteNatMappingIP(ctx context.Context, cl client.Client, ip *ipamv1alpha1.IP) error {
	var fwcfg networkingv1beta1.FirewallConfiguration

	err := cl.Get(ctx, client.ObjectKey{Name: generateNatMappingIPGwName(ip), Namespace: ip.Namespace}, &fwcfg)
	switch {
	case errors.IsNotFound(err):
		// Nothing to delete for the gateway mapping; fall through to also clean up
		// any leftover masquerade firewall configuration.
	case err != nil:
		return fmt.Errorf("unable to get the firewall configuration %s/%s: %w", ip.Namespace, generateNatMappingIPGwName(ip), err)
	default:
		if err := deleteNatMappingIPWithFirewallconfiguration(ctx, cl, ip, &fwcfg); err != nil {
			return err
		}
	}

	// Always ensure absence of masquerade firewall rules: they may have been created
	// when Spec.Masquerade was previously true, even if it is now false/nil.
	return ensureFirewallConfigurationMasqueradeAbsence(ctx, cl, ip)
}

// createOrUpdateFirewallConfiguration wraps resource.CreateOrUpdate with the standard logging
// performed on every firewall configuration managed by this controller.
func createOrUpdateFirewallConfiguration(ctx context.Context, cl client.Client,
	fwcfg *networkingv1beta1.FirewallConfiguration, mutate func() error) error {
	op, err := resource.CreateOrUpdate(ctx, cl, fwcfg, mutate)
	if op != controllerutil.OperationResultNone {
		klog.Infof("FirewallConfiguration %s/%s %s", fwcfg.Namespace, fwcfg.Name, op)
	}
	return err
}

// ensureFirewallConfigurationMasqueradeAbsence ensures the absence of the masquerade
// firewall configuration associated with the given IP: it removes the IP's NAT rules
// and deletes the FirewallConfiguration when no rules remain. Safe to call when the
// resource does not exist.
func ensureFirewallConfigurationMasqueradeAbsence(ctx context.Context, cl client.Client, ip *ipamv1alpha1.IP) error {
	fwcfg := &networkingv1beta1.FirewallConfiguration{}
	err := cl.Get(ctx, client.ObjectKey{Name: generateNatMappingIPFabricName(ip), Namespace: ip.Namespace}, fwcfg)
	switch {
	case errors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("unable to get the firewall configuration %s/%s: %w", ip.Namespace, generateNatMappingIPFabricName(ip), err)
	}
	return deleteNatMappingIPWithFirewallconfiguration(ctx, cl, ip, fwcfg)
}

// deleteNatMappingIPWithFirewallconfiguration deletes the NAT mapping for an IP in a specific firewallconfiguration.
func deleteNatMappingIPWithFirewallconfiguration(ctx context.Context, cl client.Client,
	ip *ipamv1alpha1.IP, fwcfg *networkingv1beta1.FirewallConfiguration) error {
	err := cl.Update(ctx, cleanFirewallConfiguration(fwcfg, ip))
	switch {
	case errors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("unable to update the firewall configuration %q: %w", fwcfg.Name, err)
	}

	return deleteFirewallConfiguration(ctx, cl, fwcfg)
}

func cleanFirewallConfiguration(fwcfg *networkingv1beta1.FirewallConfiguration, ip *ipamv1alpha1.IP) *networkingv1beta1.FirewallConfiguration {
	for i := range fwcfg.Spec.Table.Chains {
		chain := &fwcfg.Spec.Table.Chains[i]
		chain.Rules.NatRules = slices.DeleteFunc(chain.Rules.NatRules, func(r firewall.NatRule) bool {
			return r.Name != nil && *r.Name == ip.Name
		})
	}
	return fwcfg
}

func deleteFirewallConfiguration(ctx context.Context, cl client.Client, fwcfg *networkingv1beta1.FirewallConfiguration) error {
	allChainsVoid := true
	for i := range fwcfg.Spec.Table.Chains {
		chain := &fwcfg.Spec.Table.Chains[i]
		if len(chain.Rules.NatRules) > 0 {
			allChainsVoid = false
		}
	}

	if allChainsVoid {
		err := cl.Delete(ctx, fwcfg)
		if client.IgnoreNotFound(err) != nil {
			return err
		}
		if err == nil {
			klog.Infof("Deleted firewall configuration %s/%s", fwcfg.Namespace, fwcfg.Name)
		}
	}
	return nil
}

func mutateFirewallConfiguration(fwcfg *networkingv1beta1.FirewallConfiguration, ip *ipamv1alpha1.IP) func() error {
	return func() error {
		fwcfg.SetLabels(ForgeFirewallTargetLabelsIPMappingGw())
		enforceFirewallConfigurationSpec(fwcfg, ip)
		return nil
	}
}

func mutateFirewallConfigurationMasquerade(fwcfg *networkingv1beta1.FirewallConfiguration, ip *ipamv1alpha1.IP) func() error {
	return func() error {
		fwcfg.SetLabels(ForgeFirewallTargetLabelsIPMappingFabric())
		enforceFirewallConfigurationMasqSpec(fwcfg, ip)
		return nil
	}
}

func enforceFirewallConfigurationSpec(fwcfg *networkingv1beta1.FirewallConfiguration, ip *ipamv1alpha1.IP) {
	table := &fwcfg.Spec.Table
	table.Name = ptr.To(fmt.Sprintf("%s-%s", generateNatMappingIPGwName(ip), fwcfg.Namespace))
	table.Family = ptr.To(firewall.TableFamilyIPv4)
	enforceFirewallConfigurationChains(fwcfg, ip)
}

func enforceFirewallConfigurationMasqSpec(fwcfg *networkingv1beta1.FirewallConfiguration, ip *ipamv1alpha1.IP) {
	table := &fwcfg.Spec.Table
	table.Name = ptr.To(fmt.Sprintf("%s-%s", generateNatMappingIPFabricName(ip), fwcfg.Namespace))
	table.Family = ptr.To(firewall.TableFamilyIPv4)
	enforceFirewallConfigurationMasqChains(fwcfg, ip)
}

func enforceFirewallConfigurationChains(fwcfg *networkingv1beta1.FirewallConfiguration, ip *ipamv1alpha1.IP) {
	if fwcfg.Spec.Table.Chains == nil || len(fwcfg.Spec.Table.Chains) != 2 {
		fwcfg.Spec.Table.Chains = make([]firewall.Chain, 2)
	}
	chainPre := &fwcfg.Spec.Table.Chains[0]
	chainPre.Name = &PreroutingChainName
	chainPre.Policy = ptr.To(firewall.ChainPolicyAccept)
	chainPre.Type = firewall.ChainTypeNAT
	chainPre.Hook = &firewall.ChainHookPrerouting
	chainPre.Priority = ptr.To(firewall.ChainPriorityNATDest)
	ensureFirewallConfigurationDNATRules(&chainPre.Rules, ip)

	chainPost := &fwcfg.Spec.Table.Chains[1]
	chainPost.Name = &PostroutingChainName
	chainPost.Policy = ptr.To(firewall.ChainPolicyAccept)
	chainPost.Type = firewall.ChainTypeNAT
	chainPost.Hook = &firewall.ChainHookPostrouting
	chainPost.Priority = ptr.To(firewall.ChainPriorityNATSource)
	ensureFirewallConfigurationSNATRules(&chainPost.Rules, ip)
}

func enforceFirewallConfigurationMasqChains(fwcfg *networkingv1beta1.FirewallConfiguration, ip *ipamv1alpha1.IP) {
	if fwcfg.Spec.Table.Chains == nil || len(fwcfg.Spec.Table.Chains) != 2 {
		fwcfg.Spec.Table.Chains = make([]firewall.Chain, 2)
	}
	chainPre := &fwcfg.Spec.Table.Chains[0]
	chainPre.Name = &PreroutingChainName
	chainPre.Policy = ptr.To(firewall.ChainPolicyAccept)
	chainPre.Type = firewall.ChainTypeNAT
	chainPre.Hook = &firewall.ChainHookPrerouting
	chainPre.Priority = ptr.To(firewall.ChainPriorityNATDest)
	ensureFirewallConfigurationDNATRules(&chainPre.Rules, ip)

	chainPost := &fwcfg.Spec.Table.Chains[1]
	chainPost.Name = &PostroutingChainName
	chainPost.Policy = ptr.To(firewall.ChainPolicyAccept)
	chainPost.Type = firewall.ChainTypeNAT
	chainPost.Hook = &firewall.ChainHookPostrouting
	chainPost.Priority = ptr.To(firewall.ChainPriorityNATSource - 1)
	ensureFirewallConfigurationMasqSNATRules(&chainPost.Rules, ip)
}

func containsNATRule(rules []firewall.NatRule, to string, pos firewall.MatchPosition) bool {
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

func ensureFirewallConfigurationDNATRules(rules *firewall.RulesSet, ip *ipamv1alpha1.IP) {
	if !containsNATRule(rules.NatRules, ip.Spec.IP.String(), firewall.MatchPositionDst) {
		remappedIP := ipamutils.GetRemappedIP(ip)
		rules.NatRules = append(rules.NatRules, firewall.NatRule{
			NatType: firewall.NatTypeDestination,
			To:      ptr.To(ip.Spec.IP.String()),
			Name:    &ip.Name,
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Position: firewall.MatchPositionDst,
						Value:    remappedIP.String(),
					},
				},
			},
		})
	}
}

func ensureFirewallConfigurationSNATRules(rules *firewall.RulesSet, ip *ipamv1alpha1.IP) {
	remappedIP := ipamutils.GetRemappedIP(ip)
	if !containsNATRule(rules.NatRules, ip.Spec.IP.String(), firewall.MatchPositionSrc) {
		rules.NatRules = append(rules.NatRules, firewall.NatRule{
			NatType: firewall.NatTypeSource,
			To:      ptr.To(ip.Spec.IP.String()),
			Name:    &ip.Name,
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Position: firewall.MatchPositionSrc,
						Value:    remappedIP.String(),
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
				if rules[i].Match[j].IP != nil && rules[i].Match[j].IP.Position == firewall.MatchPositionDst &&
					rules[i].Match[j].IP.Value == dst {
					return true
				}
			}
		}
	}
	return false
}

func ensureFirewallConfigurationMasqSNATRules(rules *firewall.RulesSet,
	ip *ipamv1alpha1.IP) {
	if !containsNatRuleMasquerade(rules.NatRules, ip.Spec.IP.String()) {
		rules.NatRules = append(rules.NatRules, firewall.NatRule{
			Name:    ptr.To(ip.Name),
			NatType: firewall.NatTypeMasquerade,
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Position: firewall.MatchPositionDst,
						Value:    ip.Spec.IP.String(),
					},
				},
			},
		})
	}
}
