package discovery

const (
	TenantControlNamespaceLabel = "discovery.liqo.io/tenant-control-namespace"
	ClusterRoleLabel            = "discovery.liqo.io/cluster-role"

	ClusterIdLabel      = "discovery.liqo.io/cluster-id"
	AuthTokenLabel      = "discovery.liqo.io/auth-token"
	RemoteIdentityLabel = "discovery.liqo.io/remote-identity"
	DiscoveryTypeLabel  = "discovery.liqo.io/discovery-type"
	SearchDomainLabel   = "discovery.liqo.io/searchdomain"
	LiqoManagedLabel    = "discovery.liqo.io/liqo-managed"
	GarbageCollection   = "discovery.liqo.io/garbage-collection"
)

type DiscoveryType string

const (
	LanDiscovery             DiscoveryType = "LAN"
	WanDiscovery             DiscoveryType = "WAN"
	ManualDiscovery          DiscoveryType = "Manual"
	IncomingPeeringDiscovery DiscoveryType = "IncomingPeering"
)

type TrustMode string

const (
	TrustModeUnknown   TrustMode = "Unknown"
	TrustModeTrusted   TrustMode = "Trusted"
	TrustModeUntrusted TrustMode = "Untrusted"
)

type AuthStatus string

const (
	AuthStatusPending      AuthStatus = "Pending"
	AuthStatusAccepted     AuthStatus = "Accepted"
	AuthStatusRefused      AuthStatus = "Refused"
	AuthStatusEmptyRefused AuthStatus = "EmptyRefused"
)

const (
	LastUpdateAnnotation string = "LastUpdate"
)
