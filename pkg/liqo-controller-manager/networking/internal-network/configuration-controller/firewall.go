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
		var err error

		if fwcfg.Labels == nil {
			fwcfg.Labels = make(map[string]string)
		}
		fwcfg.SetLabels(labels.Merge(fwcfg.Labels, fabric.ForgeFirewallTargetLabels()))

		if err := controllerutil.SetOwnerReference(cfg, fwcfg, scheme); err != nil {
			return err
		}

		fwcfg.Spec.Table.Name = ptr.To(generateFirewallConfigurationName(cfg))
		fwcfg.Spec.Table.Family = ptr.To(firewallapi.TableFamilyIPv4)
		fwcfg.Spec.Table.Chains = []firewallapi.Chain{*forgeFirewallChain()}

		fwcfg.Spec.Table.Chains[0].Rules.NatRules, err = forgeFirewallNatRule(cfg, opts)
		if err != nil {
			return err
		}

		return nil
	}
}

func forgeFirewallChain() *firewallapi.Chain {
	return &firewallapi.Chain{
		Name:     ptr.To(PrePostroutingChainName),
		Type:     firewallapi.ChainTypeNAT,
		Policy:   ptr.To(firewallapi.ChainPolicyAccept),
		Priority: ptr.To(firewallapi.ChainPriorityNATSource - 1),
		Hook:     ptr.To(firewallapi.ChainHookPostrouting),
		Rules: firewallapi.RulesSet{
			NatRules: []firewallapi.NatRule{},
		},
	}
}

func forgeFirewallNatRule(cfg *networkingv1beta1.Configuration, opts *Options) (natrules []firewallapi.NatRule, err error) {
	if len(cfg.Spec.Local.CIDR.External) == 0 {
		return nil, fmt.Errorf("configuration %q has no local external CIDR", cfg.Name)
	}

	unknownSourceIP, err := ipamutils.GetUnknownSourceIP(cidrutils.GetPrimary(cfg.Spec.Local.CIDR.External).String())
	if err != nil {
		return nil, fmt.Errorf("unable to get first IP from CIDR: %w", err)
	}

	localPodCIDRs := cfg.Spec.Local.CIDR.Pod
	remotePodCIDRs := cfg.Status.Remote.CIDR.Pod
	remoteExtCIDR := cidrutils.GetPrimary(cfg.Status.Remote.CIDR.External)

	// Pod CIDR: per (local-pod, remote-pod) pair, a no-op SNAT acting as masquerade-bypass marker.
	if !opts.FullMasqueradeEnabled {
		for li := range localPodCIDRs {
			for ri := range remotePodCIDRs {
				localPodCIDRstr := localPodCIDRs[li].String()
				remotePodCIDRstr := remotePodCIDRs[ri].String()
				natrules = append(natrules, firewallapi.NatRule{
					Name: ptr.To(generatePodNatRuleName(cfg, localPodCIDRstr, remotePodCIDRstr)),
					Match: []firewallapi.Match{
						{
							Op: firewallapi.MatchOperationEq,
							IP: &firewallapi.MatchIP{Position: firewallapi.MatchPositionDst, Value: remotePodCIDRstr},
						},
						{
							Op: firewallapi.MatchOperationEq,
							IP: &firewallapi.MatchIP{Position: firewallapi.MatchPositionSrc, Value: localPodCIDRstr},
						},
					},
					NatType: firewallapi.NatTypeSource,
					To:      ptr.To(localPodCIDRstr),
				})
			}
		}
	}

	// NodePort: per remote-pod-CIDR, SNAT to unknown-source-IP for traffic not originating from any local pod CIDR.
	for ri := range remotePodCIDRs {
		remotePodCIDRStr := remotePodCIDRs[ri].String()
		rule := firewallapi.NatRule{
			Name: ptr.To(generateNodePortSvcNatRuleName(cfg, remotePodCIDRStr)),
			Match: []firewallapi.Match{
				{
					Op: firewallapi.MatchOperationEq,
					IP: &firewallapi.MatchIP{Position: firewallapi.MatchPositionDst, Value: remotePodCIDRStr},
				},
			},
			NatType: firewallapi.NatTypeSource,
			To:      ptr.To(unknownSourceIP),
		}

		// If full masquerade is enable we do the SNAT for all the traffic.
		if !opts.FullMasqueradeEnabled {
			for li := range localPodCIDRs {
				rule.Match = append(rule.Match, firewallapi.Match{
					Op: firewallapi.MatchOperationNeq,
					IP: &firewallapi.MatchIP{Position: firewallapi.MatchPositionSrc, Value: localPodCIDRs[li].String()},
				})
			}
		}
		natrules = append(natrules, rule)
	}

	// External CIDR: per (local-pod, remote-external) pair, a no-op SNAT acting as masquerade-bypass marker.
	if !opts.FullMasqueradeEnabled {
		for li := range localPodCIDRs {
			localPodCIDRStr := localPodCIDRs[li].String()
			natrules = append(natrules, firewallapi.NatRule{
				Name: ptr.To(generatePodNatRuleNameExt(cfg, localPodCIDRStr)),
				Match: []firewallapi.Match{
					{
						Op: firewallapi.MatchOperationEq,
						IP: &firewallapi.MatchIP{Position: firewallapi.MatchPositionDst, Value: remoteExtCIDR.String()},
					},
					{
						Op: firewallapi.MatchOperationEq,
						IP: &firewallapi.MatchIP{Position: firewallapi.MatchPositionSrc, Value: localPodCIDRStr},
					},
				},
				NatType: firewallapi.NatTypeSource,
				To:      ptr.To(localPodCIDRStr),
			})
		}
	}

	// NodePort: SNAT to unknown-source-IP for traffic towards remote-external-CIDR not originating from any local pod CIDR.
	rule := firewallapi.NatRule{
		Name: ptr.To(generateNodePortSvcNatRuleNameExt(cfg)),
		Match: []firewallapi.Match{
			{
				Op: firewallapi.MatchOperationEq,
				IP: &firewallapi.MatchIP{Position: firewallapi.MatchPositionDst, Value: remoteExtCIDR.String()},
			},
		},
		NatType: firewallapi.NatTypeSource,
		To:      ptr.To(unknownSourceIP),
	}

	// If full masquerade is enable we do the SNAT for all the traffic.
	if !opts.FullMasqueradeEnabled {
		for li := range localPodCIDRs {
			rule.Match = append(rule.Match, firewallapi.Match{
				Op: firewallapi.MatchOperationNeq,
				IP: &firewallapi.MatchIP{Position: firewallapi.MatchPositionSrc, Value: localPodCIDRs[li].String()},
			})
		}
	}
	natrules = append(natrules, rule)
	return natrules, nil
}

func generateFirewallConfigurationName(cfg *networkingv1beta1.Configuration) string {
	return fmt.Sprintf("%s-masquerade-bypass", cfg.Name)
}

func generatePodNatRuleName(cfg *networkingv1beta1.Configuration, localCidr, remoteCidr string) string {
	return fmt.Sprintf("podcidr-%s-%s-%s", cfg.Name, cidrutils.EscapeForNameStr(localCidr), cidrutils.EscapeForNameStr(remoteCidr))
}

func generateNodePortSvcNatRuleName(cfg *networkingv1beta1.Configuration, remoteCidr string) string {
	return fmt.Sprintf("service-nodeport-%s-%s", cfg.Name, cidrutils.EscapeForNameStr(remoteCidr))
}

func generatePodNatRuleNameExt(cfg *networkingv1beta1.Configuration, localCidr string) string {
	return fmt.Sprintf("podcidr-%s-ext-%s", cfg.Name, cidrutils.EscapeForNameStr(localCidr))
}

func generateNodePortSvcNatRuleNameExt(cfg *networkingv1beta1.Configuration) string {
	return fmt.Sprintf("service-nodeport-%s-ext", cfg.Name)
}
