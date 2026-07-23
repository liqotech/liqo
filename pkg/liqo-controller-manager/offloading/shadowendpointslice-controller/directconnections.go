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
	"errors"
	"fmt"
	"maps"

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

// EventReasonDirectConnectionNotPeered is used when a Service requests direct connections towards
// providers that were never network-peered.
const EventReasonDirectConnectionNotPeered = "DirectConnectionNotPeered"

// directPathState describes the usability of the provider-to-provider direct path the endpoints
// of a slice pair depend on.
type directPathState int

const (
	// directPathUnused: the slice takes no part in direct connections (no direct-connections
	// data and not an indirect companion).
	directPathUnused directPathState = iota
	// directPathActive: a Connection towards every direct cluster exists and is Connected.
	directPathActive
	// directPathDown: Connections exist towards all the direct clusters, but at least one is not
	// (yet) Connected. Transient: it recovers on its own and the Connection watch retriggers the
	// reconcile when it does.
	directPathDown
	// directPathNotPeered: no Connection exists at all towards some of the direct clusters — the
	// providers were never network-peered (or have been un-peered since). A misconfiguration
	// that requires operator action.
	directPathNotPeered
	// directPathDenied: this provider denies direct connections altogether (controller flag), so
	// the path is never used regardless of any peering. Their health is not even checked.
	directPathDenied
)

// directPath is the outcome of resolveDirectPath: the role of the slice in a direct/indirect
// pair and the usability of the direct path its endpoints depend on.
type directPath struct {
	state      directPathState
	isIndirect bool

	// data holds the direct-connections annotation content: for each cluster reachable through a
	// direct connection, the endpoint addresses that belong to it.
	data directconnection.ClusterAddresses

	// notPeered carries the clusters missing a Connection when state is directPathNotPeered.
	notPeered *directconnection.NotPeeredError
}

// isDirect reports whether the slice is the direct member of a pair: it carries
// direct-connections data and is not the indirect companion.
func (dp *directPath) isDirect() bool {
	return !dp.isIndirect && len(dp.data.Clusters) > 0
}

// resolveDirectPath classifies the shadow slice with respect to the direct-connections feature
// and, for the slices taking part in it, determines the usability of the direct path by checking
// the health of the Connections towards the involved clusters.
func (r *Reconciler) resolveDirectPath(ctx context.Context,
	shadowEps *offloadingv1beta1.ShadowEndpointSlice) (directPath, error) {
	//nolint:goconst // label value, not worth a shared constant
	dp := directPath{isIndirect: shadowEps.Labels[forge.IndirectEndpointSliceLabelKey] == "true"}

	if val, ok := shadowEps.Annotations[consts.DirectConnectionDataAnnotationKey]; ok {
		if err := dp.data.FromJSON([]byte(val)); err != nil {
			return dp, fmt.Errorf("failed to unmarshal direct connection data: %w", err)
		}
	}

	if !dp.isIndirect && len(dp.data.Clusters) == 0 {
		dp.state = directPathUnused
		return dp, nil
	}

	if r.DenyDirectConnections {
		dp.state = directPathDenied
		return dp, nil
	}

	var notPeered *directconnection.NotPeeredError
	switch err := directconnection.CheckConnections(ctx, r.Client, dp.data.ClusterIDs()); {
	case err == nil:
		dp.state = directPathActive
	case errors.As(err, &notPeered):
		dp.state = directPathNotPeered
		dp.notPeered = notPeered
	case errors.Is(err, directconnection.ErrConnectionsDown):
		dp.state = directPathDown
	default:
		return dp, fmt.Errorf("failed to check direct connections status: %w", err)
	}

	return dp, nil
}

// reportNotPeered surfaces the never-peered misconfiguration for a direct slice with a Warning
// event and a log line. The reconcile continues: the endpoints depending on the unpeered
// cluster(s) are excluded from the materialized slice (see dropEndpointsOfClusters), while their
// hub copies in the indirect companion keep serving traffic through the consumer, so the Service
// loses no backend. Recovery is level-triggered: the Connection created by 'liqoctl network
// connect' retriggers the reconcile through the watch.
func (r *Reconciler) reportNotPeered(ctx context.Context, shadowEps *offloadingv1beta1.ShadowEndpointSlice,
	notPeered *directconnection.NotPeeredError) {
	eventMsg := fmt.Sprintf("no direct network peering to clusters %v", notPeered.Clusters)
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

// dropEndpointsOfClusters filters out (in lockstep from both parallel slices) the endpoints that
// depend on one of the given clusters. Used for the never-peered case: those endpoints cannot be
// translated (no Configuration exists towards their cluster) nor reached directly, and dropping
// them also flushes the stale addresses of a previously working slice after an un-peering.
func dropEndpointsOfClusters(endpoints []discoveryv1.Endpoint, epClusters, toDrop []string) (
	filtered []discoveryv1.Endpoint, filteredClusters []string) {
	dropSet := make(map[string]struct{}, len(toDrop))
	for _, c := range toDrop {
		dropSet[c] = struct{}{}
	}

	filtered = make([]discoveryv1.Endpoint, 0, len(endpoints))
	filteredClusters = make([]string, 0, len(epClusters))
	for i := range endpoints {
		if _, drop := dropSet[epClusters[i]]; drop {
			continue
		}
		filtered = append(filtered, endpoints[i])
		filteredClusters = append(filteredClusters, epClusters[i])
	}
	return filtered, filteredClusters
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

// computeEndpointReady returns the Ready condition to apply to a single endpoint. The rule:
// readiness depends on the health of the PATH the endpoint is reached through, not on the slice
// it sits in. Path-independent endpoints (consumer-hosted, external) follow only the ordinary
// conditions; endpoints reached through a direct connection are ready only while it is usable;
// their hub copies in the indirect companion only when it is not (so that, per logical backend,
// exactly one representation is active at any time).
func computeEndpointReady(dp *directPath, endpointCluster string, networkReady, apiServerReady bool) bool {
	// Endpoints are ready only if both the tunnel endpoint and the API server of the foreign
	// cluster (the consumer the slice was reflected from) are ready.
	ready := networkReady && apiServerReady

	switch {
	case dp.isIndirect && len(dp.data.Clusters) == 0:
		// Companion without direct-connections data (only produced by older virtual kubelets,
		// which replicated every endpoint): fully overlapping with the direct member of the
		// pair, keep it not-ready to avoid duplicating the endpoints.
		return false

	case dp.isIndirect:
		// Hub copy of a direct-connection endpoint: serves traffic whenever the direct path is
		// not usable (down, not peered, or denied), falling back through the consumer.
		return ready && dp.state != directPathActive

	case endpointCluster != "":
		// Direct-connection endpoint on the direct slice: ready only while the direct path is
		// fully usable.
		return ready && dp.state == directPathActive

	default:
		// Path-independent endpoint (consumer-hosted, external) or plain slice: its
		// reachability never depended on the provider-to-provider connections.
		return ready
	}
}

// applyEndpointsReadiness computes and applies the Ready condition to each endpoint.
// epClusters is the parallel classification from classifyEndpoints (nil for non-direct slices:
// all endpoints are then treated as path-independent or ruled by the indirect cases).
//
// Note: an endpoint is updated only if its Ready condition is True or nil, i.e. if the foreign
// cluster sets the endpoint condition Ready to False (the backing pod is failing its readiness
// probe at the origin).
func applyEndpointsReadiness(endpoints []discoveryv1.Endpoint, epClusters []string, dp *directPath,
	networkReady, apiServerReady bool) {
	for i := range endpoints {
		cluster := ""
		if epClusters != nil {
			cluster = epClusters[i]
		}
		ready := computeEndpointReady(dp, cluster, networkReady, apiServerReady)

		endpoint := &endpoints[i]
		if endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready {
			endpoint.Conditions.Ready = &ready
		}
	}
}

// removeDirectConnectionAnnotation returns a copy of annotations without direct-connection data.
func removeDirectConnectionAnnotation(annotations map[string]string) map[string]string {
	if annotations == nil {
		return nil
	}
	filtered := maps.Clone(annotations)
	delete(filtered, consts.DirectConnectionDataAnnotationKey)
	return filtered
}
