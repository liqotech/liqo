package consts

type PeeringPhase string

const (
	PeeringPhaseNone          PeeringPhase = "None"
	PeeringPhaseAll           PeeringPhase = "All"
	PeeringPhaseEstablished   PeeringPhase = "Established"
	PeeringPhaseIncoming      PeeringPhase = "Incoming"
	PeeringPhaseOutgoing      PeeringPhase = "Outgoing"
	PeeringPhaseBidirectional PeeringPhase = "Bidirectional"
)
