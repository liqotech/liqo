package foreigncluster

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// IsIncomingJoined checks if the incoming peering has been completely established.
func IsIncomingJoined(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return foreignCluster.Status.Incoming.PeeringPhase == discoveryv1alpha1.PeeringPhaseEstablished
}

// IsOutgoingJoined checks if the outgoing peering has been completely established.
func IsOutgoingJoined(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return foreignCluster.Status.Outgoing.PeeringPhase == discoveryv1alpha1.PeeringPhaseEstablished
}

// IsIncomingEnabled checks if the incoming peering is enabled (i.e. Pending, Established or Deleting).
func IsIncomingEnabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return foreignCluster.Status.Incoming.PeeringPhase != discoveryv1alpha1.PeeringPhaseNone && foreignCluster.Status.Incoming.PeeringPhase != ""
}

// IsOutgoingEnabled checks if the outgoing peering is enabled (i.e. Pending, Established or Deleting).
func IsOutgoingEnabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return foreignCluster.Status.Outgoing.PeeringPhase != discoveryv1alpha1.PeeringPhaseNone && foreignCluster.Status.Outgoing.PeeringPhase != ""
}
