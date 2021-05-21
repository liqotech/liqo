package foreignclusteroperator

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

type desiredPeeringPhase string

const (
	desiredPeeringPhasePeering   desiredPeeringPhase = "Peering"
	desiredPeeringPhaseUnpeering desiredPeeringPhase = "Unpeering"
)

// getDesiredOutgoingPeeringState returns the desired state for the outgoing peering basing on the ForeignCluster resource.
func (r *ForeignClusterReconciler) getDesiredOutgoingPeeringState(foreignCluster *discoveryv1alpha1.ForeignCluster) desiredPeeringPhase {
	remoteNamespace := foreignCluster.Status.TenantControlNamespace.Remote
	if remoteNamespace != "" && foreignCluster.Spec.Join {
		return desiredPeeringPhasePeering
	}
	return desiredPeeringPhaseUnpeering
}
