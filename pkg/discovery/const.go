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
