package discovery

const (
	ClusterIdLabel      = "liqo.io/cluster-id"
	AuthTokenLabel      = "liqo.io/auth-token"
	RemoteIdentityLabel = "liqo.io/remote-identity"
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
