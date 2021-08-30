package consts

// PeeringPhase contains the status of the peering with a remote cluster.
type PeeringPhase string

const (
	// PeeringPhaseNone -> no pering has been established.
	PeeringPhaseNone PeeringPhase = "None"
	// PeeringPhaseAuthenticated -> an identity to interact with the remote cluster is available.
	PeeringPhaseAuthenticated PeeringPhase = "Authenticated"
	// PeeringPhaseEstablished -> the peering has been established (either incoming or outgoing).
	PeeringPhaseEstablished PeeringPhase = "Established"
	// PeeringPhaseIncoming -> an incoming peering has been established.
	PeeringPhaseIncoming PeeringPhase = "Incoming"
	// PeeringPhaseOutgoing -> an outgoing peering has been established.
	PeeringPhaseOutgoing PeeringPhase = "Outgoing"
	// PeeringPhaseBidirectional -> both incoming and outgoing peerings have been established.
	PeeringPhaseBidirectional PeeringPhase = "Bidirectional"
)
