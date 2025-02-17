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
//

package peer

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/common"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// GatewayType represents the type of network gateway (Client or Server).
type GatewayType string

const (
	// GatewayClientType indicates that the gateway acts as a client.
	GatewayClientType GatewayType = "Client"
	// GatewayServerType indicates that the gateway acts as a server.
	GatewayServerType GatewayType = "Server"
)

// CIDRInfo contains info about CIDR and its remapping.
type CIDRInfo struct {
	Remote   networkingv1beta1.ClusterConfigCIDR  `json:"remote"`
	Remapped *networkingv1beta1.ClusterConfigCIDR `json:"remapped,omitempty"`
}

// GatewayInfo contains info about the network gateway.
type GatewayInfo struct {
	Address []string    `json:"address"`
	Port    int32       `json:"port"`
	Role    GatewayType `json:"role"`
}

// Network contains some info and the status of the network between the local cluster and a peer.
type Network struct {
	Status  common.ModuleStatus `json:"status"`
	Alerts  []string            `json:"alerts,omitempty"`
	CIDRs   CIDRInfo
	Gateway GatewayInfo `json:"gateway"`
}

// NetworkChecker collects some info about the status of the network between the local cluster and the active peers.
type NetworkChecker struct {
	info.CheckerCommon

	// In this case data is a mapping between ClusterID and Network info
	data map[liqov1beta1.ClusterID]Network
}

// Collect some info about the status of the network between the local cluster and the active peers.
func (nc *NetworkChecker) Collect(ctx context.Context, options info.Options) {
	nc.data = map[liqov1beta1.ClusterID]Network{}
	for clusterID := range options.ClustersInfo {
		peerNetwork := Network{}

		// Collect info about the status of the network module
		nc.collectStatusInfo(clusterID, options.ClustersInfo, &peerNetwork)

		// If the module is disabled we do not need to collect any additional info
		if peerNetwork.Status != common.ModuleDisabled {
			// Get the network CIDRs
			config, err := getters.GetConfigurationByClusterID(ctx, options.CRClient, clusterID, corev1.NamespaceAll)
			if err != nil {
				nc.AddCollectionError(fmt.Errorf("unable to get network configuration for cluster %q: %w", clusterID, err))
			} else {
				peerNetwork.CIDRs = CIDRInfo{
					Remote:   config.Spec.Remote.CIDR,
					Remapped: &config.Status.Remote.CIDR,
				}
			}

			// Collect info about the gateway
			if err := nc.collectGatewayInfo(ctx, options.CRClient, clusterID, &peerNetwork); err != nil {
				nc.AddCollectionError(fmt.Errorf("unable to get network gateway info for cluster %q: %w", clusterID, err))
			}
		}

		nc.data[clusterID] = peerNetwork
	}
}

// FormatForClusterID returns the collected data for the specified clusterID using a user friendly output.
func (nc *NetworkChecker) FormatForClusterID(clusterID liqov1beta1.ClusterID, options info.Options) string {
	if data, ok := nc.data[clusterID]; ok {
		main := output.NewRootSection()
		main.AddEntry("Status", common.FormatStatus(data.Status))
		if data.Status != common.ModuleDisabled {
			// Show alerts if any
			if len(data.Alerts) > 0 {
				main.AddEntryWarning("Alerts", data.Alerts...)
			}

			// Print info about CIDR
			cidrSection := main.AddSection("CIDR")

			remoteCIDRSection := cidrSection.AddSection("Remote")
			if data.CIDRs.Remapped != nil {
				remoteCIDRSection.AddEntry("Pod CIDR",
					fmt.Sprintf("%s → Remapped to %s", joinCidrs(data.CIDRs.Remote.Pod), joinCidrs(data.CIDRs.Remapped.Pod)))
				remoteCIDRSection.AddEntry("External CIDR",
					fmt.Sprintf("%s → Remapped to %s", joinCidrs(data.CIDRs.Remote.External), joinCidrs(data.CIDRs.Remapped.External)))
			} else {
				remoteCIDRSection.AddEntry("Pod CIDR", joinCidrs(data.CIDRs.Remote.Pod))
				remoteCIDRSection.AddEntry("External CIDR", joinCidrs(data.CIDRs.Remote.External))
			}

			// Print info about Gateway
			gatewaySection := main.AddSection("Gateway")
			gatewaySection.AddEntry("Role", string(data.Gateway.Role))
			gatewaySection.AddEntry("Address", data.Gateway.Address...)
			gatewaySection.AddEntry("Port", fmt.Sprint(data.Gateway.Port))
		}

		return main.SprintForBox(options.Printer)
	}
	return ""
}

// GetDataByClusterID returns the data collected by the checker for the cluster with the give ClusterID.
func (nc *NetworkChecker) GetDataByClusterID(clusterID liqov1beta1.ClusterID) (interface{}, error) {
	if res, ok := nc.data[clusterID]; ok {
		return res, nil
	}
	return nil, fmt.Errorf("no data collected for cluster %q", clusterID)
}

// GetID returns the id of the section collected by the checker.
func (nc *NetworkChecker) GetID() string {
	return "network"
}

// GetTitle returns the title of the section collected by the checker.
func (nc *NetworkChecker) GetTitle() string {
	return "Network"
}

// collectStatusInfo collects the info about the status of the network between the local cluster and the one with the given ClusterID.
func (nc *NetworkChecker) collectStatusInfo(clusterID liqov1beta1.ClusterID,
	clusterInfo map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster, peerNetwork *Network) {
	cluster := clusterInfo[clusterID]
	peerNetwork.Status, peerNetwork.Alerts = common.CheckModuleStatusAndAlerts(cluster.Status.Modules.Networking)
}

// collectGatewayInfo collects the info about the local gateway connected to the peer cluster gateway.
func (nc *NetworkChecker) collectGatewayInfo(ctx context.Context, cl client.Client, clusterID liqov1beta1.ClusterID, peerNetwork *Network) error {
	gwServer, gwClient, err := getters.GetGatewaysByClusterID(ctx, cl, clusterID)
	if err != nil {
		return err
	}

	switch {
	case gwClient != nil && gwServer != nil:
		return fmt.Errorf("multiple Gateways found")
	case gwClient != nil:
		// We are reflecting the status of the peer cluster. So, if locally there is a client, the peer
		// cluester has a server.
		peerNetwork.Gateway.Role = GatewayServerType
		peerNetwork.Gateway.Address = gwClient.Spec.Endpoint.Addresses
		peerNetwork.Gateway.Port = gwClient.Spec.Endpoint.Port
	case gwServer != nil:
		peerNetwork.Gateway.Role = GatewayClientType
		if gwServer.Status.Endpoint != nil {
			peerNetwork.Gateway.Address = gwServer.Status.Endpoint.Addresses
			peerNetwork.Gateway.Port = gwServer.Status.Endpoint.Port
		}
	default:
		return fmt.Errorf("no gateways found")
	}

	return nil
}

func joinCidrs(cidrs []networkingv1beta1.CIDR) string {
	cidrsString := make([]string, len(cidrs))
	for i := range cidrs {
		cidrsString[i] = cidrs[i].String()
	}
	return strings.Join(cidrsString, ",")
}
