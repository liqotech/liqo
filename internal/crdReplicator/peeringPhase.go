package crdreplicator

import (
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

func (c *Controller) getPeeringPhase(clusterID string) consts.PeeringPhase {
	c.peeringPhasesMutex.RLock()
	defer c.peeringPhasesMutex.RUnlock()
	if c.peeringPhases == nil {
		return consts.PeeringPhaseNone
	}
	if phase, ok := c.peeringPhases[clusterID]; ok {
		return phase
	} else {
		return consts.PeeringPhaseNone
	}
}

func (c *Controller) setPeeringPhase(clusterID string, phase consts.PeeringPhase) {
	c.peeringPhasesMutex.Lock()
	defer c.peeringPhasesMutex.Unlock()
	if c.peeringPhases == nil {
		c.peeringPhases = map[string]consts.PeeringPhase{}
	}
	c.peeringPhases[clusterID] = phase
}

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

func isReplicationEnabled(peeringPhase consts.PeeringPhase, resource resourceToReplicate) bool {
	switch resource.peeringPhase {
	case consts.PeeringPhaseNone:
		return false
	case consts.PeeringPhaseAny:
		return true
	case consts.PeeringPhaseBidirectional:
		return peeringPhase == consts.PeeringPhaseBidirectional
	case consts.PeeringPhaseIncoming:
		return peeringPhase == consts.PeeringPhaseBidirectional || peeringPhase == consts.PeeringPhaseIncoming
	case consts.PeeringPhaseOutgoing:
		return peeringPhase == consts.PeeringPhaseBidirectional || peeringPhase == consts.PeeringPhaseOutgoing
	case consts.PeeringPhaseEstablished:
		return peeringPhase == consts.PeeringPhaseBidirectional || peeringPhase == consts.PeeringPhaseIncoming || peeringPhase == consts.PeeringPhaseOutgoing
	default:
		klog.Info("unknown peeringPhase %v", resource.peeringPhase)
		return false
	}
}
