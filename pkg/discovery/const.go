package discovery

const (
	// TenantNamespaceLabel used to mark the tenant namespaces.
	TenantNamespaceLabel = "discovery.liqo.io/tenant-namespace"

	// ClusterIDLabel used as key to indicate which cluster a resource is referenced to.
	ClusterIDLabel = "discovery.liqo.io/cluster-id"
	// AuthTokenLabel used to mark secrets containing an auth token.
	AuthTokenLabel = "discovery.liqo.io/auth-token"
	// RemoteIdentityLabel used to mark secrets containing an remote identity.
	RemoteIdentityLabel = "discovery.liqo.io/remote-identity"
	// DiscoveryTypeLabel used to mark the discovery type.
	DiscoveryTypeLabel = "discovery.liqo.io/discovery-type"
	// SearchDomainLabel used to mark the search domain linked to the foreign cluster.
	SearchDomainLabel = "discovery.liqo.io/searchdomain"
	// GarbageCollection is finalizer used to mark the resources managed by Liqo that needs an explicit garbage collection.
	GarbageCollection = "discovery.liqo.io/garbage-collection"
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

// TrustMode indicates if the authentication service is exposed with a trusted certificate.
type TrustMode string

const (
	// TrustModeUnknown value.
	TrustModeUnknown TrustMode = "Unknown"
	// TrustModeTrusted value.
	TrustModeTrusted TrustMode = "Trusted"
	// TrustModeUntrusted value.
	TrustModeUntrusted TrustMode = "Untrusted"
)

const (
	// LastUpdateAnnotation marks the last update time of a ForeignCluster resource, needed by the garbage collection.
	LastUpdateAnnotation string = "LastUpdate"
)
