package foreigncluster

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// GetPeeringPhase returns the peering phase of a given ForignCluster CR.
func GetPeeringPhase(fc *discoveryv1alpha1.ForeignCluster) consts.PeeringPhase {
	authenticated := IsAuthenticated(fc)
	incoming := IsIncomingEnabled(fc)
	outgoing := IsOutgoingEnabled(fc)

	switch {
	case incoming && outgoing:
		return consts.PeeringPhaseBidirectional
	case incoming:
		return consts.PeeringPhaseIncoming
	case outgoing:
		return consts.PeeringPhaseOutgoing
	case authenticated:
		return consts.PeeringPhaseAuthenticated
	default:
		return consts.PeeringPhaseNone
	}
}
