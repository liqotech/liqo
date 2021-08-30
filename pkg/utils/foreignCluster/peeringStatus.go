package foreigncluster

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

// IsAuthenticated checks if the identity has been accepted by the remote cluster.
func IsAuthenticated(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.AuthenticationStatusCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusEstablished
}

// IsIncomingJoined checks if the incoming peering has been completely established.
func IsIncomingJoined(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.IncomingPeeringCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusEstablished
}

// IsOutgoingJoined checks if the outgoing peering has been completely established.
func IsOutgoingJoined(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.OutgoingPeeringCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusEstablished
}

// IsIncomingEnabled checks if the incoming peering is enabled (i.e. Pending, Established or Deleting).
func IsIncomingEnabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.IncomingPeeringCondition)
	return curPhase != discoveryv1alpha1.PeeringConditionStatusNone && curPhase != ""
}

// IsOutgoingEnabled checks if the outgoing peering is enabled (i.e. Pending, Established or Deleting).
func IsOutgoingEnabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.OutgoingPeeringCondition)
	return curPhase != discoveryv1alpha1.PeeringConditionStatusNone && curPhase != ""
}
