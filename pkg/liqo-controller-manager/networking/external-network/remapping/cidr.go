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

package remapping

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// CIDRType is the type of the CIDR.
type CIDRType string

const (
	// PodCIDR is the CIDR type for the pod CIDR.
	PodCIDR CIDRType = "PodCIDR"
	// ExternalCIDR is the CIDR type for the external CIDR.
	ExternalCIDR CIDRType = "ExternalCIDR"
)

// CreateOrUpdateNatMappingCIDR creates or updates the NAT mapping for a CIDR type.
func CreateOrUpdateNatMappingCIDR(ctx context.Context, cl client.Client, opts *Options,
	cfg *networkingv1beta1.Configuration, scheme *runtime.Scheme, cidrtype CIDRType) error {
	var tableCIDRName string
	switch cidrtype {
	case PodCIDR:
		tableCIDRName = TablePodCIDRName
	case ExternalCIDR:
		tableCIDRName = TableExternalCIDRName
	}
	fwcfg := &networkingv1beta1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", cfg.Name, tableCIDRName),
			Namespace: cfg.Namespace,
		},
	}

	klog.Infof("Creating firewall configuration %q for %q", fwcfg.Name, cidrtype)

	if _, err := resource.CreateOrUpdate(
		ctx, cl, fwcfg,
		mutateCIDRFirewallConfiguration(fwcfg, cfg, opts, scheme, cidrtype),
	); err != nil {
		return err
	}

	klog.Infof("Firewall configuration %q for %q created", fwcfg.Name, cidrtype)

	return nil
}

func mutateCIDRFirewallConfiguration(fwcfg *networkingv1beta1.FirewallConfiguration, cfg *networkingv1beta1.Configuration,
	opts *Options, scheme *runtime.Scheme, cidrtype CIDRType) func() error {
	return func() error {
		if cfg.Labels == nil {
			return fmt.Errorf("configuration %q has no labels", cfg.Name)
		}
		remoteClusterID := cfg.Labels[string(consts.RemoteClusterID)]
		fwcfg.SetLabels(ForgeFirewallTargetLabels(remoteClusterID))
		fwcfg.Spec = forgeCIDRFirewallConfigurationSpec(cfg, opts, cidrtype)
		return controllerutil.SetOwnerReference(cfg, fwcfg, scheme)
	}
}

func forgeCIDRFirewallConfigurationSpec(cfg *networkingv1beta1.Configuration, opts *Options,
	cidrtype CIDRType) networkingv1beta1.FirewallConfigurationSpec {
	var tableCIDRName string
	switch cidrtype {
	case PodCIDR:
		tableCIDRName = TablePodCIDRName
	case ExternalCIDR:
		tableCIDRName = TableExternalCIDRName
	}

	return networkingv1beta1.FirewallConfigurationSpec{
		Table: firewall.Table{
			Name:   &tableCIDRName,
			Family: ptr.To(firewall.TableFamilyIPv4),
			Chains: []firewall.Chain{
				forgeCIDRFirewallConfigurationDNATChain(cfg, opts, cidrtype),
				forgeCIDRFirewallConfigurationSNATChain(cfg, opts, cidrtype),
			},
		},
	}
}

func forgeCIDRFirewallConfigurationDNATChain(cfg *networkingv1beta1.Configuration, opts *Options, cidrtype CIDRType) firewall.Chain {
	return firewall.Chain{
		Name:     &DNATChainName,
		Policy:   ptr.To(firewall.ChainPolicyAccept),
		Type:     ptr.To(firewall.ChainTypeNAT),
		Hook:     &firewall.ChainHookPrerouting,
		Priority: &firewall.ChainPriorityNATDest,
		Rules: firewall.RulesSet{
			NatRules: forgeCIDRFirewallConfigurationDNATRules(cfg, opts, cidrtype),
		},
	}
}

func forgeCIDRFirewallConfigurationSNATChain(cfg *networkingv1beta1.Configuration,
	opts *Options, cidrtype CIDRType) firewall.Chain {
	return firewall.Chain{
		Name:     &SNATChainName,
		Policy:   ptr.To(firewall.ChainPolicyAccept),
		Type:     ptr.To(firewall.ChainTypeNAT),
		Hook:     &firewall.ChainHookPostrouting,
		Priority: &firewall.ChainPriorityNATSource,
		Rules: firewall.RulesSet{
			NatRules: forgeCIDRFirewallConfigurationSNATRules(cfg, opts, cidrtype),
		},
	}
}

func forgeCIDRFirewallConfigurationDNATRules(cfg *networkingv1beta1.Configuration, opts *Options, cidrtype CIDRType) []firewall.NatRule {
	var remoteCIDR, remoteRemapCIDR string
	switch cidrtype {
	case PodCIDR:
		remoteCIDR = cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.Pod).String()
		remoteRemapCIDR = cidrutils.GetPrimary(cfg.Status.Remote.CIDR.Pod).String()
	case ExternalCIDR:
		remoteCIDR = cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.External).String()
		remoteRemapCIDR = cidrutils.GetPrimary(cfg.Status.Remote.CIDR.External).String()
	}
	return []firewall.NatRule{
		{
			NatType: firewall.NatTypeDestination,
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Value:    remoteRemapCIDR,
						Position: firewall.MatchPositionDst,
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
						Value:    tunnel.TunnelInterfaceName,
						Position: firewall.MatchDevPositionIn,
					},
				},
			},
			To: ptr.To(remoteCIDR),
		},
	}
}

func forgeCIDRFirewallConfigurationSNATRules(cfg *networkingv1beta1.Configuration,
	opts *Options, cidrtype CIDRType) []firewall.NatRule {
	var localCIDR, remoteRemapCIDR string
	switch cidrtype {
	case PodCIDR:
		localCIDR = cidrutils.GetPrimary(cfg.Spec.Local.CIDR.Pod).String()
		remoteRemapCIDR = cidrutils.GetPrimary(cfg.Status.Remote.CIDR.Pod).String()
	case ExternalCIDR:
		localCIDR = cidrutils.GetPrimary(cfg.Spec.Local.CIDR.External).String()
		remoteRemapCIDR = cidrutils.GetPrimary(cfg.Status.Remote.CIDR.External).String()
	}

	return []firewall.NatRule{
		{
			NatType: firewall.NatTypeSource,
			To:      ptr.To(remoteRemapCIDR),
			Match: []firewall.Match{
				{
					Op: firewall.MatchOperationNeq,
					Dev: &firewall.MatchDev{
						Value:    opts.DefaultInterfaceName,
						Position: firewall.MatchDevPositionOut,
					},
				},
				{
					Op: firewall.MatchOperationEq,
					IP: &firewall.MatchIP{
						Value:    localCIDR,
						Position: firewall.MatchPositionSrc,
					},
				},
				{
					Op: firewall.MatchOperationEq,
					Dev: &firewall.MatchDev{
						Value:    tunnel.TunnelInterfaceName,
						Position: firewall.MatchDevPositionIn,
					},
				},
			},
		},
	}
}
