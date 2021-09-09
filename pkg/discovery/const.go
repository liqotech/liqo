// Copyright 2019-2021 The Liqo Authors
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

package discovery

const (
	// TenantNamespaceLabel used to mark the tenant namespaces.
	TenantNamespaceLabel = "discovery.liqo.io/tenant-namespace"

	// ClusterIDLabel used as key to indicate which cluster a resource is referenced to.
	ClusterIDLabel = "discovery.liqo.io/cluster-id"
	// AuthTokenLabel used to mark secrets containing an auth token.
	AuthTokenLabel = "discovery.liqo.io/auth-token"
	// DiscoveryTypeLabel used to mark the discovery type.
	DiscoveryTypeLabel = "discovery.liqo.io/discovery-type"
	// SearchDomainLabel used to mark the search domain linked to the foreign cluster.
	SearchDomainLabel = "discovery.liqo.io/searchdomain"
)

// Type indicates how a ForeignCluster has been discovered.
type Type string

const (
	// LanDiscovery value.
	LanDiscovery Type = "LAN"
	// WanDiscovery value.
	WanDiscovery Type = "WAN"
	// ManualDiscovery value.
	ManualDiscovery Type = "Manual"
	// IncomingPeeringDiscovery value.
	IncomingPeeringDiscovery Type = "IncomingPeering"
)

const (
	// LastUpdateAnnotation marks the last update time of a ForeignCluster resource, needed by the garbage collection.
	LastUpdateAnnotation string = "LastUpdate"
)
