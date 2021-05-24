package crdreplicator

import (
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// getPeeringPhase returns the peering phase for a cluster given its clusterID.
func (c *Controller) getPeeringPhase(clusterID string) consts.PeeringPhase {
	c.peeringPhasesMutex.RLock()
	defer c.peeringPhasesMutex.RUnlock()
	if c.peeringPhases == nil {
		return consts.PeeringPhaseNone
	}
	if phase, ok := c.peeringPhases[clusterID]; ok {
		return phase
	}
	return consts.PeeringPhaseNone
}

// setPeeringPhase sets the peering phase for a given clusterID.
func (c *Controller) setPeeringPhase(clusterID string, phase consts.PeeringPhase) {
	c.peeringPhasesMutex.Lock()
	defer c.peeringPhasesMutex.Unlock()
	if c.peeringPhases == nil {
		c.peeringPhases = map[string]consts.PeeringPhase{}
	}
	c.peeringPhases[clusterID] = phase
}

// getPeeringPhase returns the peering phase for a fiver ForignCluster CR.
func getPeeringPhase(fc *discoveryv1alpha1.ForeignCluster) consts.PeeringPhase {
	if fc.Status.Incoming.Joined && fc.Status.Outgoing.Joined {
		return consts.PeeringPhaseBidirectional
	}
	if fc.Status.Incoming.Joined {
		return consts.PeeringPhaseIncoming
	}
	if fc.Status.Outgoing.Joined {
		return consts.PeeringPhaseOutgoing
	}
	return consts.PeeringPhaseNone
}

// isReplicationEnabled indicates if the replication has to be enabled for a given peeringPhase
// and a given CRD.
func isReplicationEnabled(peeringPhase consts.PeeringPhase, resource configv1alpha1.Resource) bool {
	switch resource.PeeringPhase {
	case consts.PeeringPhaseNone:
		return false
	case consts.PeeringPhaseAll:
		return true
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
		klog.Info("unknown peeringPhase %v", resource.PeeringPhase)
		return false
	}
}
