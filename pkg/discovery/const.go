package discovery

const (
	// TenantControlNamespaceLabel used to mark the tenant control namespaces
	TenantControlNamespaceLabel = "discovery.liqo.io/tenant-control-namespace"
	// ClusterRoleLabel used all the Liqo cluster roles
	ClusterRoleLabel = "discovery.liqo.io/cluster-role"

	// ClusterIDLabel used as key to indicate which cluster a resource is referenced to
	ClusterIDLabel = "discovery.liqo.io/cluster-id"
	// AuthTokenLabel used to mark secrets containing an auth token
	AuthTokenLabel = "discovery.liqo.io/auth-token"
	// RemoteIdentityLabel used to mark secrets containing an remote identity
	RemoteIdentityLabel = "discovery.liqo.io/remote-identity"
	// DiscoveryTypeLabel used to mark the discovery type
	DiscoveryTypeLabel = "discovery.liqo.io/discovery-type"
	// DiscoveryTypeLabel used to mark the search domain linked to the foreign cluster
	SearchDomainLabel = "discovery.liqo.io/searchdomain"
	// RemoteIdentityLabel used to mark the resources managed by Liqo
	LiqoManagedLabel = "discovery.liqo.io/liqo-managed"
	// Finalizer used to mark the resources managed by Liqo that needs an explicit garbage collection
	GarbageCollection = "discovery.liqo.io/garbage-collection"
)

// DiscoveryType indicates how a ForeignCluster has been discovered
type DiscoveryType string

const (
	// LanDiscovery value
	LanDiscovery DiscoveryType = "LAN"
	// WanDiscovery value
	WanDiscovery DiscoveryType = "WAN"
	// ManualDiscovery value
	ManualDiscovery DiscoveryType = "Manual"
	// IncomingPeeringDiscovery value
	IncomingPeeringDiscovery DiscoveryType = "IncomingPeering"
)

// TrustMode indicates if the authentication service is exposed with a trusted certificate
type TrustMode string

const (
	// TrustModeUnknown value
	TrustModeUnknown TrustMode = "Unknown"
	// TrustModeTrusted value
	TrustModeTrusted TrustMode = "Trusted"
	// TrustModeUntrusted value
	TrustModeUntrusted TrustMode = "Untrusted"
)

type AuthStatus string

const (
	// AuthStatusPending value
	AuthStatusPending AuthStatus = "Pending"
	// AuthStatusAccepted value
	AuthStatusAccepted AuthStatus = "Accepted"
	// AuthStatusRefused value
	AuthStatusRefused AuthStatus = "Refused"
	// AuthStatusEmptyRefused value
	AuthStatusEmptyRefused AuthStatus = "EmptyRefused"
)

const (
	// LastUpdateAnnotation marks the last update time of a ForeignCluster resource, needed by the garbage collection
	LastUpdateAnnotation string = "LastUpdate"
)
