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

package configurationcontroller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/fabric"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

func (r *ConfigurationReconciler) ensureFirewallConfiguration(ctx context.Context,
	cfg *networkingv1beta1.Configuration, opts *Options) error {
	firewall := &networkingv1beta1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateFirewallConfigurationName(cfg),
			Namespace: cfg.GetNamespace(),
		},
	}
	_, err := resource.CreateOrUpdate(ctx, r.Client, firewall, forgeMutateFirewallConfiguration(firewall, cfg, r.Scheme, opts))
	if err != nil {
		return err
	}
	return nil
}

func forgeMutateFirewallConfiguration(fwcfg *networkingv1beta1.FirewallConfiguration,
	cfg *networkingv1beta1.Configuration, scheme *runtime.Scheme, opts *Options) func() error {
	return func() error {
		if fwcfg.Labels == nil {
			fwcfg.Labels = make(map[string]string)
		}
		fwcfg.SetLabels(labels.Merge(fwcfg.Labels, fabric.ForgeFirewallTargetLabels()))

		if err := controllerutil.SetOwnerReference(cfg, fwcfg, scheme); err != nil {
			return err
		}

		fwcfg.Spec.Table.Name = ptr.To(generateFirewallConfigurationName(cfg))

		fwcfg.Spec.Table.Family = ptr.To(firewallapi.TableFamilyIPv4)

		if fwcfg.Spec.Table.Chains == nil || len(fwcfg.Spec.Table.Chains) != 1 {
			fwcfg.Spec.Table.Chains = []firewallapi.Chain{*forgeFirewallChain()}
		}

		if !isNatRuleAlreadyPresentInChain(cfg, &fwcfg.Spec.Table.Chains[0]) {
			if fwcfg.Spec.Table.Chains[0].Rules.NatRules == nil {
				fwcfg.Spec.Table.Chains[0].Rules.NatRules = []firewallapi.NatRule{}
			}
			rules, err := forgeFirewallNatRule(cfg, opts)
			if err != nil {
				return err
			}
			fwcfg.Spec.Table.Chains[0].Rules.NatRules = append(fwcfg.Spec.Table.Chains[0].Rules.NatRules, rules...)
		}
		return nil
	}
}

func forgeFirewallChain() *firewallapi.Chain {
	return &firewallapi.Chain{
		Name:     ptr.To(PrePostroutingChainName),
		Type:     ptr.To(firewallapi.ChainTypeNAT),
		Policy:   ptr.To(firewallapi.ChainPolicyAccept),
		Priority: ptr.To(firewallapi.ChainPriorityNATSource - 1),
		Hook:     ptr.To(firewallapi.ChainHookPostrouting),
		Rules: firewallapi.RulesSet{
			NatRules: []firewallapi.NatRule{},
		},
	}
}

func forgeFirewallNatRule(cfg *networkingv1beta1.Configuration, opts *Options) (natrules []firewallapi.NatRule, err error) {
	unknownSourceIP, err := ipamutils.GetUnknownSourceIP(cidrutils.GetPrimary(cfg.Spec.Local.CIDR.External).String())
	if err != nil {
		return nil, fmt.Errorf("unable to get first IP from CIDR: %w", err)
	}

	// Pod CIDR
	if !opts.FullMasqueradeEnabled {
		natrules = append(natrules, firewallapi.NatRule{
			Name: ptr.To(generatePodNatRuleName(cfg)),
			Match: []firewallapi.Match{
				{
					Op: firewallapi.MatchOperationEq,
					IP: &firewallapi.MatchIP{
						Position: firewallapi.MatchPositionDst,
						Value:    cidrutils.GetPrimary(cfg.Status.Remote.CIDR.Pod).String(),
					},
				},
				{
					Op: firewallapi.MatchOperationEq,
					IP: &firewallapi.MatchIP{
						Position: firewallapi.MatchPositionSrc,
						Value:    cidrutils.GetPrimary(cfg.Spec.Local.CIDR.Pod).String(),
					},
				},
			},
			NatType: firewallapi.NatTypeSource,
			To:      ptr.To(cidrutils.GetPrimary(cfg.Spec.Local.CIDR.Pod).String()),
		})
	}

	natrules = append(natrules, firewallapi.NatRule{
		Name: ptr.To(generateNodePortSvcNatRuleName(cfg)),
		Match: []firewallapi.Match{
			{
				Op: firewallapi.MatchOperationEq,
				IP: &firewallapi.MatchIP{
					Position: firewallapi.MatchPositionDst,
					Value:    cidrutils.GetPrimary(cfg.Status.Remote.CIDR.Pod).String(),
				},
			},
		},
		NatType: firewallapi.NatTypeSource,
		To:      ptr.To(unknownSourceIP),
	})
	if !opts.FullMasqueradeEnabled {
		natrules[1].Match = append(natrules[1].Match, firewallapi.Match{
			Op: firewallapi.MatchOperationNeq,
			IP: &firewallapi.MatchIP{
				Position: firewallapi.MatchPositionSrc,
				Value:    cidrutils.GetPrimary(cfg.Spec.Local.CIDR.Pod).String(),
			},
		})
	}

	// External CIDR
	if !opts.FullMasqueradeEnabled {
		natrules = append(natrules, firewallapi.NatRule{
			Name: ptr.To(generatePodNatRuleNameExt(cfg)),
			Match: []firewallapi.Match{
				{
					Op: firewallapi.MatchOperationEq,
					IP: &firewallapi.MatchIP{
						Position: firewallapi.MatchPositionDst,
						Value:    cidrutils.GetPrimary(cfg.Status.Remote.CIDR.External).String(),
					},
				},
				{
					Op: firewallapi.MatchOperationEq,
					IP: &firewallapi.MatchIP{
						Position: firewallapi.MatchPositionSrc,
						Value:    cidrutils.GetPrimary(cfg.Spec.Local.CIDR.Pod).String(),
					},
				},
			},
			NatType: firewallapi.NatTypeSource,
			To:      ptr.To(cidrutils.GetPrimary(cfg.Spec.Local.CIDR.Pod).String()),
		})
	}

	natrules = append(natrules, firewallapi.NatRule{
		Name: ptr.To(generateNodePortSvcNatRuleNameExt(cfg)),
		Match: []firewallapi.Match{
			{
				Op: firewallapi.MatchOperationEq,
				IP: &firewallapi.MatchIP{
					Position: firewallapi.MatchPositionDst,
					Value:    cidrutils.GetPrimary(cfg.Status.Remote.CIDR.External).String(),
				},
			},
		},
		NatType: firewallapi.NatTypeSource,
		To:      ptr.To(unknownSourceIP),
	})
	if !opts.FullMasqueradeEnabled {
		natrules[3].Match = append(natrules[3].Match, firewallapi.Match{
			Op: firewallapi.MatchOperationNeq,
			IP: &firewallapi.MatchIP{
				Position: firewallapi.MatchPositionSrc,
				Value:    cidrutils.GetPrimary(cfg.Spec.Local.CIDR.Pod).String(),
			},
		})
	}
	return natrules, nil
}

func generateFirewallConfigurationName(cfg *networkingv1beta1.Configuration) string {
	return fmt.Sprintf("%s-masquerade-bypass", cfg.Name)
}

func generatePodNatRuleName(cfg *networkingv1beta1.Configuration) string {
	return fmt.Sprintf("podcidr-%s", cfg.Name)
}

func generateNodePortSvcNatRuleName(cfg *networkingv1beta1.Configuration) string {
	return fmt.Sprintf("service-nodeport-%s", cfg.Name)
}

func generatePodNatRuleNameExt(cfg *networkingv1beta1.Configuration) string {
	return fmt.Sprintf("podcidr-%s-ext", cfg.Name)
}

func generateNodePortSvcNatRuleNameExt(cfg *networkingv1beta1.Configuration) string {
	return fmt.Sprintf("service-nodeport-%s-ext", cfg.Name)
}

func isNatRuleAlreadyPresentInChain(cfg *networkingv1beta1.Configuration, chain *firewallapi.Chain) bool {
	natRuleNames := []string{generatePodNatRuleName(cfg), generateNodePortSvcNatRuleName(cfg)}
	if chain.Rules.NatRules == nil {
		return false
	}
	for _, rule := range chain.Rules.NatRules {
		for _, natRuleName := range natRuleNames {
			if rule.Name != nil && *rule.Name == natRuleName {
				return true
			}
		}
	}
	return false
}
