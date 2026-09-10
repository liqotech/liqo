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
	"context"
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

const (
	// gwExtMark is the fwmark value used to tag traffic arriving on Geneve interfaces (liqo.*).
	// It allows the gw-ext RouteConfiguration to match on FwMark + Dst instead of Iif + Dst,
	// collapsing N*R rules to R rules while still preventing routing loops (packets arriving
	// on the WireGuard interface liqo-tunnel are not marked and do not match).
	// The mark is set per-packet in the prerouting chain (not via conntrack) so it is available
	// for route lookup and does not leak to return traffic.
	//
	// The value 0xFF00 is chosen to avoid collision with the internal-network mark allocator
	// (pkg/liqo-controller-manager/networking/internal-network/route/mark.go), which assigns
	// sequential marks starting from 1, one per node. A high value ensures no overlap even in
	// very large clusters.
	gwExtMark = 0xFF00

	// gwExtGenevePrefix is the prefix shared by all Geneve interfaces created by Liqo.
	gwExtGenevePrefix = "liqo."

	// GwNodeMark is the fwmark value used to tag traffic arriving on WireGuard tunnel interfaces (liqo-tunnel*).
	GwNodeMark = 0xFE00

	// GwNodeTunnelPrefix is the prefix shared by all WireGuard tunnel interfaces.
	GwNodeTunnelPrefix = tunnel.TunnelInterfaceName
)

// GenerateRouteConfigurationName generates the name of the RouteConfiguration object.
func GenerateRouteConfigurationName(cfg *networkingv1beta1.Configuration) string {
	return fmt.Sprintf("%s-gw-ext", cfg.Name)
}

// GenerateFirewallConfigurationName generates the name of the FirewallConfiguration object.
func GenerateFirewallConfigurationName(cfg *networkingv1beta1.Configuration, suffix string) string {
	return fmt.Sprintf("%s-%s", cfg.Name, suffix)
}

// GetRemoteClusterID returns the remote cluster ID of the Configuration.
func GetRemoteClusterID(cfg *networkingv1beta1.Configuration) (liqov1beta1.ClusterID, error) {
	if cfg.GetLabels() == nil {
		return "", fmt.Errorf("configuration %s/%s has no labels", cfg.Namespace, cfg.Name)
	}
	remoteID, ok := cfg.GetLabels()[consts.RemoteClusterID]
	if !ok {
		return "", fmt.Errorf("configuration %s/%s has no remote cluster ID label", cfg.Namespace, cfg.Name)
	}
	return liqov1beta1.ClusterID(remoteID), nil
}

// enforceRouteConfigurationPresence creates or updates a RouteConfiguration object and its
// associated FirewallConfiguration.
// It also creates a FirewallConfiguration to mark incoming traffic arriving from WireGuard tunnel interfaces.
func enforceRouteConfigurationPresence(ctx context.Context, cl client.Client, scheme *runtime.Scheme,
	cfg *networkingv1beta1.Configuration) error {
	remoteClusterID, err := GetRemoteClusterID(cfg)
	if err != nil {
		return err
	}

	mode, err := GetGatewayMode(ctx, cl, remoteClusterID)
	if err != nil {
		return err
	}
	// If the Gateway is not already present, we are not able to understand if it will be a server or a client
	if mode == "" {
		return nil
	}

	remoteInterfaceIP, err := tunnel.GetRemoteInterfaceIP(mode)
	if err != nil {
		return err
	}

	// Ensure the FirewallConfiguration that marks traffic arriving on Geneve interfaces.
	fwcfgExt := &networkingv1beta1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateFirewallConfigurationName(cfg, "gw-ext"),
			Namespace: cfg.Namespace,
		},
	}
	if _, err = resource.CreateOrUpdate(ctx, cl, fwcfgExt,
		forgeMutateFirewallConfiguration(cfg, fwcfgExt, scheme, remoteClusterID, "gw-ext-mark", gwExtGenevePrefix, gwExtMark)); err != nil {
		return fmt.Errorf("ensuring firewall configuration %q: %w", fwcfgExt.Name, err)
	}
	// Ensure the FirewallConfiguration that marks traffic arriving on Wireguard tunnels.
	fwcfgNode := &networkingv1beta1.FirewallConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateFirewallConfigurationName(cfg, "gw-node"),
			Namespace: cfg.Namespace,
		},
	}
	if _, err = resource.CreateOrUpdate(ctx, cl, fwcfgNode,
		forgeMutateFirewallConfiguration(cfg, fwcfgNode, scheme, remoteClusterID, "gw-node-mark", GwNodeTunnelPrefix, GwNodeMark)); err != nil {
		return fmt.Errorf("ensuring firewall configuration %q: %w", fwcfgNode.Name, err)
	}

	routecfg := &networkingv1beta1.RouteConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateRouteConfigurationName(cfg),
			Namespace: cfg.Namespace,
		},
	}
	_, err = resource.CreateOrUpdate(ctx, cl, routecfg,
		forgeMutateRouteConfiguration(cfg, routecfg, scheme, remoteClusterID, remoteInterfaceIP))
	return err
}

// forgeMutateFirewallConfiguration mutates a FirewallConfiguration object that marks traffic
// arriving on interfaces matching ifacePrefix with a constant fwmark. The mark is set directly
// on the packet (not via conntrack) in the prerouting chain at mangle priority, so it is
// available for ip rule route lookup. This is per-packet, not per-connection, preventing
// return traffic on liqo-tunnel from being marked.
func forgeMutateFirewallConfiguration(cfg *networkingv1beta1.Configuration,
	fwcfg *networkingv1beta1.FirewallConfiguration, scheme *runtime.Scheme,
	remoteClusterID liqov1beta1.ClusterID, chainName string, ifacePrefix string, mark uint32) func() error {
	return func() error {
		if err := controllerutil.SetOwnerReference(cfg, fwcfg, scheme); err != nil {
			return err
		}

		fwcfg.Labels = gateway.ForgeFirewallExternalTargetLabels(string(remoteClusterID))

		markValue := fmt.Sprintf("%d", mark)

		fwcfg.Spec = networkingv1beta1.FirewallConfigurationSpec{
			Table: firewall.Table{
				Name:   ptr.To(fmt.Sprintf("%s-%s", cfg.Name, chainName)),
				Family: ptr.To(firewall.TableFamilyIPv4),
				Chains: []firewall.Chain{
					{
						// Prerouting chain at mangle priority: set the packet fwmark directly for any
						// packet arriving on a liqo.* interface. Runs before route lookup so ip rule
						// can match on FwMark. The WireGuard interface (liqo-tunnel) uses a dash, not
						// a dot, so it does not match the "liqo." prefix.
						Name:     ptr.To(fmt.Sprintf("%s-%s", cfg.Name, chainName)),
						Type:     firewall.ChainTypeFilter,
						Policy:   ptr.To(firewall.ChainPolicyAccept),
						Hook:     ptr.To(firewall.ChainHookPrerouting),
						Priority: &firewall.ChainPriorityMangle,
						Rules: firewall.RulesSet{
							FilterRules: []firewall.FilterRule{
								{
									Name: ptr.To(chainName),
									Match: []firewall.Match{
										{
											Op: firewall.MatchOperationEq,
											Dev: &firewall.MatchDev{
												Value:    ifacePrefix,
												Position: firewall.MatchDevPositionIn,
												Wildcard: true,
											},
										},
									},
									Action: firewall.ActionSetMetaMark,
									Value:  ptr.To(markValue),
								},
							},
						},
					},
				},
			},
		}
		return nil
	}
}

// forgeMutateRouteConfiguration mutates a RouteConfiguration object.
func forgeMutateRouteConfiguration(cfg *networkingv1beta1.Configuration,
	routecfg *networkingv1beta1.RouteConfiguration, scheme *runtime.Scheme,
	remoteClusterID liqov1beta1.ClusterID,
	remoteInterfaceIP string) func() error {
	return func() error {
		var err error

		if err = controllerutil.SetOwnerReference(cfg, routecfg, scheme); err != nil {
			return err
		}

		routecfg.ObjectMeta.Labels = gateway.ForgeRouteExternalTargetLabels(string(remoteClusterID))

		routecfg.Spec = networkingv1beta1.RouteConfigurationSpec{
			Table: networkingv1beta1.Table{
				Name: cfg.Name,
			},
		}

		remoteCIDRs := slices.Concat(cfg.Spec.Remote.CIDR.Pod, cfg.Spec.Remote.CIDR.External)
		mark := gwExtMark
		for j := range remoteCIDRs {
			dst := &remoteCIDRs[j]
			routecfg.Spec.Table.Rules = append(routecfg.Spec.Table.Rules, networkingv1beta1.Rule{
				FwMark: &mark,
				Dst:    dst,
				Routes: []networkingv1beta1.Route{
					{
						Dst: dst,
						Gw:  ptr.To(networkingv1beta1.IP(remoteInterfaceIP)),
					},
				},
			})
		}
		return nil
	}
}

// GetGatewayMode returns the mode of the Gateway related to the Configuration.
func GetGatewayMode(ctx context.Context, cl client.Client, remoteClusterID liqov1beta1.ClusterID) (gateway.Mode, error) {
	gwserver, gwclient, err := getters.GetGatewaysByClusterID(ctx, cl, remoteClusterID)
	if err != nil {
		return "", err
	}

	switch {
	case gwclient == nil && gwserver == nil:
		return "", nil
	case gwclient != nil && gwserver != nil:
		return "", fmt.Errorf("multiple Gateways found for cluster %s", remoteClusterID)
	case gwclient == nil && gwserver != nil:
		return gateway.ModeServer, nil
	case gwclient != nil && gwserver == nil:
		return gateway.ModeClient, nil
	}

	return "", fmt.Errorf("unable to determine Gateway mode for cluster %s", remoteClusterID)
}
