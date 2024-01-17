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

package configurationcontroller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
	"github.com/liqotech/liqo/pkg/fabric"
)

func (r *ConfigurationReconciler) ensureFirewallConfiguration(ctx context.Context, cfg *networkingv1alpha1.Configuration) error {
	firewall := &networkingv1alpha1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateFirewallConfigurationName(cfg),
			Namespace: cfg.GetNamespace(),
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, firewall, forgeMutateFirewallConfiguration(firewall, cfg, r.Scheme))
	if err != nil {
		return err
	}
	return nil
}

func forgeMutateFirewallConfiguration(fwcfg *networkingv1alpha1.FirewallConfiguration,
	cfg *networkingv1alpha1.Configuration, scheme *runtime.Scheme) func() error {
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

		if !isNatRuleAlreadyPresentInChain(generateNatRuleName(cfg), &fwcfg.Spec.Table.Chains[0]) {
			if fwcfg.Spec.Table.Chains[0].Rules.NatRules == nil {
				fwcfg.Spec.Table.Chains[0].Rules.NatRules = []firewallapi.NatRule{}
			}
			fwcfg.Spec.Table.Chains[0].Rules.NatRules = append(fwcfg.Spec.Table.Chains[0].Rules.NatRules, *forgeFirewallNatRule(cfg))
		}
		return nil
	}
}

func forgeFirewallChain() *firewallapi.Chain {
	return &firewallapi.Chain{
		Name:     ptr.To("pre-postrouting"),
		Type:     ptr.To(firewallapi.ChainTypeNAT),
		Policy:   ptr.To(firewallapi.ChainPolicyAccept),
		Priority: ptr.To(firewallapi.ChainPriorityNATSource - 1),
		Hook:     ptr.To(firewallapi.ChainHookPostrouting),
		Rules: firewallapi.RulesSet{
			NatRules: []firewallapi.NatRule{},
		},
	}
}

func forgeFirewallNatRule(cfg *networkingv1alpha1.Configuration) *firewallapi.NatRule {
	return &firewallapi.NatRule{
		Name: ptr.To(generateNatRuleName(cfg)),
		Match: []firewallapi.Match{
			{
				Op: firewallapi.MatchOperationEq,
				IP: &firewallapi.MatchIP{
					Position: firewallapi.MatchIPPositionDst,
					Value:    cfg.Status.Remote.CIDR.Pod.String(),
				},
			},
		},
		NatType: firewallapi.NatTypeSource,
		To:      ptr.To(cfg.Spec.Local.CIDR.Pod.String()),
	}
}

func generateFirewallConfigurationName(cfg *networkingv1alpha1.Configuration) string {
	return fmt.Sprintf("masquerade-bypass-%s", cfg.Name)
}

func generateNatRuleName(cfg *networkingv1alpha1.Configuration) string {
	return fmt.Sprintf("podcidr-%s", cfg.Name)
}

func isNatRuleAlreadyPresentInChain(natRuleName string, chain *firewallapi.Chain) bool {
	if chain.Rules.NatRules == nil {
		return false
	}
	for _, rule := range chain.Rules.NatRules {
		if rule.Name != nil && *rule.Name == natRuleName {
			return true
		}
	}
	return false
}
