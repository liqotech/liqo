package consts

// PeeringPhase contains the status of the peering with a remote cluster.
type PeeringPhase string

const (
	// PeeringPhaseNone no pering has been established.
	PeeringPhaseNone PeeringPhase = "None"
	// PeeringPhaseAll indicates that we have not be in any specific peering phase.
	PeeringPhaseAll PeeringPhase = "All"
	// PeeringPhaseEstablished the peering has been established.
	PeeringPhaseEstablished PeeringPhase = "Established"
	// PeeringPhaseIncoming an incoming peering has been established.
	PeeringPhaseIncoming PeeringPhase = "Incoming"
	// PeeringPhaseOutgoing an outgoing peering has been established.
	PeeringPhaseOutgoing PeeringPhase = "Outgoing"
	// PeeringPhaseBidirectional both incoming and outgoing peerings has been established.
	PeeringPhaseBidirectional PeeringPhase = "Bidirectional"
)
