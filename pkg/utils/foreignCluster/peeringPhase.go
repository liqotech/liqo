package foreigncluster

import (
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
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

// IsReplicationEnabled indicates if the replication has to be enabled for a given peeringPhase
// and a given CRD.
func IsReplicationEnabled(peeringPhase consts.PeeringPhase, resource *configv1alpha1.Resource) bool {
	switch resource.PeeringPhase {
	case consts.PeeringPhaseNone:
		return false
	case consts.PeeringPhaseAuthenticated:
		return peeringPhase != consts.PeeringPhaseNone
	case consts.PeeringPhaseBidirectional:
		return peeringPhase == consts.PeeringPhaseBidirectional
	case consts.PeeringPhaseIncoming:
		return peeringPhase == consts.PeeringPhaseBidirectional || peeringPhase == consts.PeeringPhaseIncoming
	case consts.PeeringPhaseOutgoing:
		return peeringPhase == consts.PeeringPhaseBidirectional || peeringPhase == consts.PeeringPhaseOutgoing
	case consts.PeeringPhaseEstablished:
		bidirectional := peeringPhase == consts.PeeringPhaseBidirectional
		incoming := peeringPhase == consts.PeeringPhaseIncoming
		outgoing := peeringPhase == consts.PeeringPhaseOutgoing
		return bidirectional || incoming || outgoing
	default:
		klog.Warning("Unknown peering phase %v", resource.PeeringPhase)
		return false
	}
}
