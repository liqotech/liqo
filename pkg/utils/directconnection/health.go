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

package directconnection

import (
	"context"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	networkingutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/utils"
)

// ClusterHealth reports the usability of the direct connection towards a single cluster.
type ClusterHealth int

const (
	// ClusterNotPeered means that no Connection resource exists at all towards the cluster: the
	// providers were never network-peered, or have been un-peered since. A misconfiguration that
	// requires operator action. It is the zero value, so an unknown cluster is never usable.
	ClusterNotPeered ClusterHealth = iota
	// ClusterNotConfigured means that a Connection exists, but the network Configuration towards
	// the cluster is missing, or its remapped CIDRs are not (yet) in sync with its spec. The
	// addresses of that cluster cannot be translated, so the direct path cannot be used whatever
	// the Connection says. Transient: a network peering is being established, updated or removed.
	ClusterNotConfigured
	// ClusterDown means that a Connection exists but is not (yet) Connected (Connecting, Error): a
	// transient state expected to recover on its own, needing no operator action.
	ClusterDown
	// ClusterConnected means that the direct connection towards the cluster is established and
	// usable.
	ClusterConnected
)

// Health maps each queried cluster ID to the usability of its direct connection.
//
// Health is deliberately per cluster rather than aggregated: a single EndpointSlice can carry
// endpoints hosted on several providers, and the readiness of each endpoint depends only on the
// connection towards the provider that hosts it. Aggregating would make one degraded provider
// force every other endpoint of the same Service onto the fallback path.
type Health map[string]ClusterHealth

// Usable reports whether traffic can be routed over the direct connection towards clusterID.
// An unknown cluster is never usable.
func (h Health) Usable(clusterID string) bool {
	return h[clusterID] == ClusterConnected
}

// AllUsable reports whether every recorded direct connection is usable (false if none is).
func (h Health) AllUsable() bool {
	for clusterID := range h {
		if !h.Usable(clusterID) {
			return false
		}
	}
	return len(h) > 0
}

// Clusters returns, sorted, the clusters in the given state.
func (h Health) Clusters(state ClusterHealth) []string {
	var clusters []string
	for clusterID, s := range h {
		if s == state {
			clusters = append(clusters, clusterID)
		}
	}
	slices.Sort(clusters)
	return clusters
}

// CheckConnections reports the health of the direct connection towards each of the given clusters:
// whether a Connection resource labeled with that cluster ID exists and is Connected, and whether
// the network Configuration towards that cluster is ready to translate its addresses.
//
// The check is fail-safe: callers use it to decide which EndpointSlice of a direct/indirect pair
// carries ready endpoints, and falling back to the indirect (hub-and-spoke) path is always safe,
// while routing to an unverified direct path is not. Accordingly, every cluster it cannot prove
// usable is reported as unusable.
func CheckConnections(ctx context.Context, cl client.Client, clusterIDs []string) (Health, error) {
	if len(clusterIDs) == 0 {
		return nil, nil
	}

	var connections networkingv1beta1.ConnectionList
	if err := cl.List(ctx, &connections); err != nil {
		return nil, err
	}
	status := make(map[string]networkingv1beta1.ConnectionStatusValue, len(connections.Items))
	for i := range connections.Items {
		conn := &connections.Items[i]
		if remoteID := conn.Labels[consts.RemoteClusterID]; remoteID != "" {
			status[remoteID] = conn.Status.Value
		}
	}

	var configurations networkingv1beta1.ConfigurationList
	if err := cl.List(ctx, &configurations); err != nil {
		return nil, err
	}
	configured := make(map[string]bool, len(configurations.Items))
	for i := range configurations.Items {
		cfg := &configurations.Items[i]
		if remoteID := cfg.Labels[consts.RemoteClusterID]; remoteID != "" && networkingutils.AreConfigurationNetworkCIDRsConfigured(cfg) {
			configured[remoteID] = true
		}
	}

	health := make(Health, len(clusterIDs))
	for _, clusterID := range clusterIDs {
		switch value, present := status[clusterID]; {
		case !present:
			health[clusterID] = ClusterNotPeered
		case !configured[clusterID]:
			health[clusterID] = ClusterNotConfigured
		case value == networkingv1beta1.Connected:
			health[clusterID] = ClusterConnected
		default:
			health[clusterID] = ClusterDown
		}
	}

	return health, nil
}
