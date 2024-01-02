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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
	"github.com/liqotech/liqo/pkg/gateway"
)

// CreateOrUpdateNatMappingPodCIDR creates or updates the NAT mapping for the POD CIDR.
func CreateOrUpdateNatMappingPodCIDR(ctx context.Context, cl client.Client,
	cfg *networkingv1alpha1.Configuration, scheme *runtime.Scheme, opts *Options) error {
	fwcfg := &networkingv1alpha1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cfg.Name, TablePodCIDRName),
			Namespace: cfg.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(
		ctx, cl, fwcfg,
		mutatePodCIDRFirewallConfiguration(fwcfg, cfg, opts, scheme),
	)
	return err
}

func mutatePodCIDRFirewallConfiguration(fwcfg *networkingv1alpha1.FirewallConfiguration,
	cfg *networkingv1alpha1.Configuration, opts *Options, scheme *runtime.Scheme) func() error {
	return func() error {
		fwcfg.SetLabels(ForgeFirewallTargetLabels(opts.GwOptions.RemoteClusterID))
		fwcfg.Spec = forgePodCIDRFirewallConfigurationSpec(cfg, opts)
		return gateway.SetOwnerReferenceWithMode(opts.GwOptions, fwcfg, scheme)
	}
}

func forgePodCIDRFirewallConfigurationSpec(cfg *networkingv1alpha1.Configuration,
	opts *Options) networkingv1alpha1.FirewallConfigurationSpec {
	return networkingv1alpha1.FirewallConfigurationSpec{
		Table: firewall.Table{
			Name:   &TablePodCIDRName,
			Family: ptr.To(firewall.TableFamilyIPv4),
			Chains: []firewall.Chain{
				forgePodCIDRFirewallConfigurationDNATChain(cfg, opts),
				forgePodCIDRFirewallConfigurationSNATChain(cfg, opts),
			},
		},
	}
}

func forgePodCIDRFirewallConfigurationDNATChain(cfg *networkingv1alpha1.Configuration,
	opts *Options) firewall.Chain {
	return firewall.Chain{
		Name:     &DNATChainName,
		Policy:   ptr.To(firewall.ChainPolicyAccept),
		Type:     ptr.To(firewall.ChainTypeNAT),
		Hook:     &firewall.ChainHookPrerouting,
		Priority: &firewall.ChainPriorityNATDest,
		Rules: firewall.RulesSet{
			NatRules: forgePodCIDRFirewallConfigurationDNATRules(cfg, opts),
		},
	}
}

func forgePodCIDRFirewallConfigurationSNATChain(cfg *networkingv1alpha1.Configuration,
	opts *Options) firewall.Chain {
	return firewall.Chain{
		Name:     &SNATChainName,
		Policy:   ptr.To(firewall.ChainPolicyAccept),
		Type:     ptr.To(firewall.ChainTypeNAT),
		Hook:     &firewall.ChainHookPostrouting,
		Priority: &firewall.ChainPriorityNATSource,
		Rules: firewall.RulesSet{
			NatRules: forgePodCIDRFirewallConfigurationSNATRules(cfg, opts),
		},
	}
}

func forgePodCIDRFirewallConfigurationDNATRules(cfg *networkingv1alpha1.Configuration,
	opts *Options) []firewall.NatRule {
	return []firewall.NatRule{
		{
			NatType: firewall.NatTypeDestination,
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Value:    cfg.Spec.Local.CIDR.Pod.String(),
						Position: firewall.MatchIPPositionSrc,
					},
				},
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Value:    cfg.Status.Remote.CIDR.Pod.String(),
						Position: firewall.MatchIPPositionDst,
					},
				},
				{
					Op: firewall.MatchOperationNeq,
					Dev: &firewall.MatchDev{
						Value:    opts.DefaultInterfaceName,
						Position: firewall.MatchDevPositionIn,
					},
				},
				{
					Op: firewall.MatchOperationNeq,
					Dev: &firewall.MatchDev{
						Value:    opts.GwOptions.TunnelInterfaceName,
						Position: firewall.MatchDevPositionIn,
					},
				},
			},
			To: ptr.To(cfg.Spec.Remote.CIDR.Pod.String()),
		},
	}
}

func forgePodCIDRFirewallConfigurationSNATRules(cfg *networkingv1alpha1.Configuration,
	opts *Options) []firewall.NatRule {
	return []firewall.NatRule{
		{
			NatType: firewall.NatTypeDestination,
			To:      ptr.To(cfg.Status.Remote.CIDR.Pod.String()),
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Value:    cfg.Spec.Local.CIDR.Pod.String(),
						Position: firewall.MatchIPPositionDst,
					},
				},
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Value:    cfg.Spec.Remote.CIDR.Pod.String(),
						Position: firewall.MatchIPPositionSrc,
					},
				},
				{
					Op: firewall.MatchOperationEq,
					Dev: &firewall.MatchDev{
						Value:    opts.GwOptions.TunnelInterfaceName,
						Position: firewall.MatchDevPositionIn,
					},
				},
			},
		},
	}
}
