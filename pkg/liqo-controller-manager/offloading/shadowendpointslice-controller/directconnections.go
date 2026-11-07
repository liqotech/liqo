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

package shadowendpointslicectrl

import (
	"context"
	"fmt"
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/directconnection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// This file groups everything the controller does for the direct-connections feature: endpointslices
// of Services annotated for direct connections come in pairs. The direct slice carries ALL the
// endpoints (the ones reachable through provider-to-provider connections plus the path-independent
// ones, e.g. hosted on the consumer); the -indirect companion carries only the hub-and-spoke copies
// of the direct-connection endpoints. Readiness is computed per endpoint from the health of its
// path.

const (
	// EventReasonDirectConnectionNotPeered is used when a Service requests direct connections towards
	// providers that were never network-peered.
	EventReasonDirectConnectionNotPeered = "DirectConnectionNotPeered"
	// EventReasonDirectConnectionRemapFailed is used when the address of a direct-connection endpoint
	// cannot be translated through the Configuration of the provider hosting it.
	EventReasonDirectConnectionRemapFailed = "DirectConnectionRemapFailed"
)

// directPath is the outcome of resolveDirectPath: the role of the slice in a direct/indirect pair,
// and the health of the direct connection towards each cluster its endpoints depend on.
type directPath struct {
	isIndirect bool

	// denied reports that this provider refuses direct connections altogether (controller flag),
	// so no direct path is ever used and the connections are not even checked.
	denied bool

	// data holds the direct-connections annotation content of THIS slice: for each cluster
	// reachable through a direct connection, the addresses that belong to it. The addresses are
	// the ones the slice itself carries — untranslated on the direct slice, already on the
	// consumer path on the indirect companion — so that either slice can attribute its own
	// endpoints to a cluster.
	data directconnection.ClusterAddresses

	// health reports the usability of the direct connection towards each cluster in data.
	// Nil when the path is unused or denied, which leaves every cluster unusable.
	health directconnection.Health
}

// isDirect reports whether the slice is the direct member of a pair: it carries
// direct-connections data and is not the indirect companion.
func (dp *directPath) isDirect() bool {
	return !dp.isIndirect && len(dp.data.Clusters) > 0
}

// usable reports whether the direct connection towards clusterID can carry traffic.
func (dp *directPath) usable(clusterID string) bool {
	return !dp.denied && dp.health.Usable(clusterID)
}

// allUsable reports whether every direct connection this slice depends on can carry traffic.
func (dp *directPath) allUsable() bool {
	return !dp.denied && dp.health.AllUsable()
}

// resolveDirectPath classifies the shadow slice with respect to the direct-connections feature
// and, for the slices taking part in it, determines the usability of the direct path by checking
// the health of the Connections towards the involved clusters.
func (r *Reconciler) resolveDirectPath(ctx context.Context,
	shadowEps *offloadingv1beta1.ShadowEndpointSlice) (directPath, error) {
	dp := directPath{isIndirect: shadowEps.Labels[forge.IndirectEndpointSliceLabelKey] == forge.IndirectEndpointSliceLabelValue}

	if val, ok := shadowEps.Annotations[consts.DirectConnectionDataAnnotationKey]; ok {
		if err := dp.data.FromJSON([]byte(val)); err != nil {
			return dp, fmt.Errorf("failed to unmarshal direct connection data: %w", err)
		}
	}

	// The slice takes no part in direct connections, or this provider denies them outright: in
	// both cases no connection is usable, and there is nothing to check.
	if (!dp.isIndirect && len(dp.data.Clusters) == 0) || r.DenyDirectConnections {
		dp.denied = r.DenyDirectConnections
		return dp, nil
	}

	health, err := directconnection.CheckConnections(ctx, r.Client, dp.data.ClusterIDs())
	if err != nil {
		return dp, fmt.Errorf("failed to check direct connections status: %w", err)
	}
	dp.health = health

	return dp, nil
}

// reportNotPeered surfaces the never-peered misconfiguration for a direct slice with a Warning
// event and a log line. The reconcile continues: the endpoints depending on the unpeered
// cluster(s) are excluded from the materialized slice (see Reconcile), while their
// hub copies in the indirect companion keep serving traffic through the consumer, so the Service
// loses no backend. Recovery is level-triggered: the Connection created by 'liqoctl network
// connect' retriggers the reconcile through the watch.
func (r *Reconciler) reportNotPeered(ctx context.Context, shadowEps *offloadingv1beta1.ShadowEndpointSlice,
	notPeered []string) {
	eventMsg := fmt.Sprintf("no direct network peering to clusters %v", notPeered)
	klog.Warningf("shadowendpointslice %q: %s", klog.KObj(shadowEps), eventMsg)
	r.Recorder.Event(r.eventTargetFor(ctx, shadowEps), corev1.EventTypeWarning, EventReasonDirectConnectionNotPeered, eventMsg)
}

// classifyEndpoints returns, for each endpoint, the ID of the direct cluster it depends on (the
// one its address belongs to according to the direct-connections data), or the empty string for
// path-independent endpoints (e.g. hosted on the consumer, or external): their reachability does
// not depend on any provider-to-provider connection.
//
// It must run BEFORE the endpoints are translated, since the index matches the original addresses.
func classifyEndpoints(endpoints []discoveryv1.Endpoint, index *directconnection.AddressIndex) []string {
	clusters := make([]string, len(endpoints))
	for i := range endpoints {
		for _, addr := range endpoints[i].Addresses {
			if clusterID, found := index.LookupClusterID(addr); found {
				clusters[i] = clusterID
				break
			}
		}
	}
	return clusters
}

// dropEndpoints filters out, in lockstep from both parallel slices, the endpoints for which drop
// returns true, given their position.
func dropEndpoints(endpoints []discoveryv1.Endpoint, epClusters []string, drop func(i int) bool) (
	filtered []discoveryv1.Endpoint, filteredClusters []string) {
	filtered = make([]discoveryv1.Endpoint, 0, len(endpoints))
	filteredClusters = make([]string, 0, len(epClusters))
	for i := range endpoints {
		if drop(i) {
			continue
		}
		filtered = append(filtered, endpoints[i])
		filteredClusters = append(filteredClusters, epClusters[i])
	}
	return filtered, filteredClusters
}

// reportRemapFailed surfaces, with a Warning event and a log line per endpoint, the direct-connection
// endpoints whose address cannot be translated through the Configuration of the provider hosting it
// (typically an address outside the pod CIDRs that provider advertises). Those endpoints are excluded
// from the materialized slice, while the rest of the slice is reconciled as usual.
func (r *Reconciler) reportRemapFailed(ctx context.Context, shadowEps *offloadingv1beta1.ShadowEndpointSlice, failures map[int]error) {
	positions := slices.Sorted(maps.Keys(failures))
	for _, i := range positions {
		klog.Warningf("shadowendpointslice %q: excluding endpoint %d: %v", klog.KObj(shadowEps), i, failures[i])
	}
	eventMsg := fmt.Sprintf("%d direct-connection endpoint(s) excluded, as their address cannot be remapped: %v",
		len(failures), failures[positions[0]])
	r.Recorder.Event(r.eventTargetFor(ctx, shadowEps), corev1.EventTypeWarning, EventReasonDirectConnectionRemapFailed, eventMsg)
}

// eventTargetFor returns the object to record direct-connections events on:
// the reflected Service the slice belongs to, so that the event is propagated back also to the
// consumer cluster;
// or the ShadowEndpointSlice itself when the Service cannot be resolved.
func (r *Reconciler) eventTargetFor(ctx context.Context, shadowEps *offloadingv1beta1.ShadowEndpointSlice) client.Object {
	svcName := shadowEps.Labels[discoveryv1.LabelServiceName]
	if svcName == "" {
		return shadowEps
	}

	var svc corev1.Service
	if err := r.Get(ctx, types.NamespacedName{Name: svcName, Namespace: shadowEps.Namespace}, &svc); err != nil {
		return shadowEps
	}
	return &svc
}

// readinessConditions gathers the slice-wide conditions the readiness of an endpoint may depend on,
// according to the path the endpoint is reached through.
type readinessConditions struct {
	// apiServerReady reports that the API server of the consumer the slice was reflected from is
	// reachable, so that the slice content is fresh. Every endpoint requires it.
	apiServerReady bool
	// networkReady reports that the network of the consumer peering is established, or disabled
	// altogether. Path-independent endpoints require it, as any reflected endpoint always did.
	networkReady bool
	// consumerPath reports that the hub-and-spoke path through the consumer exists and is up: the
	// consumer peering has the networking module enabled, and established. The companion copies
	// are reached through it, so they require it.
	consumerPath bool
}

// computeEndpointReady returns the Ready condition to apply to a single endpoint. The rule:
// readiness depends on the health of the PATH the endpoint is reached through, and that path is
// the direct connection towards the ONE cluster hosting it — not the aggregate health of every
// cluster the slice happens to reference. Path-independent endpoints (consumer-hosted, external)
// follow only the ordinary conditions; an endpoint reached through a direct connection is ready
// only while that connection is usable; its hub copy in the indirect companion only while it is
// not, and the consumer path is up — so that, per logical backend, at most one representation is
// active at any time, and one is whenever a path can carry the traffic.
func computeEndpointReady(dp *directPath, endpointCluster string, c readinessConditions) bool {
	switch {
	case dp.isIndirect && len(dp.data.Clusters) == 0:
		// Companion without direct-connections data (only produced by older virtual kubelets,
		// which replicated every endpoint): fully overlapping with the direct member of the
		// pair, keep it not-ready to avoid duplicating the endpoints.
		return false

	case dp.isIndirect && endpointCluster == "":
		// Companion endpoint that cannot be attributed to a cluster, because the slice was
		// written by a virtual kubelet predating the per-endpoint attribution (its annotation
		// lists the untranslated addresses, which never match the ones the companion carries).
		// Fall back to the slice-wide rule of those versions: serve while any of the direct
		// paths is unusable. It may double-serve the backend of a healthy provider, but it never
		// leaves the backend of a degraded one without a ready copy.
		return c.consumerPath && c.apiServerReady && !dp.allUsable()

	case dp.isIndirect:
		// Hub copy of a direct-connection endpoint: serves traffic whenever the direct path
		// towards ITS cluster is not usable, falling back through the consumer — provided that
		// such a path exists at all.
		return c.consumerPath && c.apiServerReady && !dp.usable(endpointCluster)

	case endpointCluster != "":
		// Direct-connection endpoint on the direct slice: ready only while the direct path
		// towards its own cluster is usable. That path does not traverse the consumer, so the
		// state of the consumer peering network does not matter.
		return c.apiServerReady && dp.usable(endpointCluster)

	default:
		// Path-independent endpoint (consumer-hosted, external) or plain slice: its
		// reachability never depended on the provider-to-provider connections.
		return c.networkReady && c.apiServerReady
	}
}

// applyEndpointsReadiness computes and applies the Ready condition to each endpoint.
// epClusters is the parallel classification from classifyEndpoints, one entry per endpoint.
//
// Note: an endpoint is updated only if its Ready condition is True or nil, i.e. if the foreign
// cluster sets the endpoint condition Ready to False (the backing pod is failing its readiness
// probe at the origin).
func applyEndpointsReadiness(endpoints []discoveryv1.Endpoint, epClusters []string, dp *directPath, c readinessConditions) {
	for i := range endpoints {
		cluster := ""
		if i < len(epClusters) {
			cluster = epClusters[i]
		}
		ready := computeEndpointReady(dp, cluster, c)

		endpoint := &endpoints[i]
		if endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready {
			endpoint.Conditions.Ready = &ready
		}
	}
}
