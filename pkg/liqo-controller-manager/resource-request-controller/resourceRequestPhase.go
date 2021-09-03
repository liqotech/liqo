package resourcerequestoperator

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

type resourceRequestPhase string

const (
	allowResourceRequestPhase    resourceRequestPhase = "Allow"
	denyResourceRequestPhase     resourceRequestPhase = "Deny"
	deletingResourceRequestPhase resourceRequestPhase = "Deleting"
)

// getResourceRequestPhase returns the phase associated with a resource request. It is:
// * "Deleting" if the deletion timestamp is set or the related offer has been withdrawn.
// * "Allow" if the incoming peering is enabled in the ForeignCluster or by the ClusterConfig.
// * "Deny" in the other cases (no ForeignCluster, incoming peering disabled, ...)
func (r *ResourceRequestReconciler) getResourceRequestPhase(
	foreignCluster *discoveryv1alpha1.ForeignCluster,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (resourceRequestPhase, error) {
	if !resourceRequest.GetDeletionTimestamp().IsZero() || !resourceRequest.Spec.WithdrawalTimestamp.IsZero() {
		return deletingResourceRequestPhase, nil
	}

	if foreignclusterutils.AllowIncomingPeering(foreignCluster, r.EnableIncomingPeering) {
		return allowResourceRequestPhase, nil
	}
	return denyResourceRequestPhase, nil
}
