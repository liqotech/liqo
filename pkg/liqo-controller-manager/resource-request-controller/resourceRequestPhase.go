package resourcerequestoperator

import (
	"context"

	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

type resourceRequestPhase string

const (
	allowResourceRequestPhase    resourceRequestPhase = "Allow"
	denyResourceRequestPhase     resourceRequestPhase = "Deny"
	deletingResourceRequestPhase resourceRequestPhase = "Deleting"
)

// getResourceRequestPhase returns the resourceRequestPhase of a resource request. It is:
// * "Deleting" if the deleteion timestamp is set or the related offer has been withdrawn.
// * "Allow" if the incoming peering is enabled for ForeignCluster o by the ClusterConfig.
// * "Deny" in the other cases (no ForeignCluster, incoming peering disabled, ...)
func (r *ResourceRequestReconciler) getResourceRequestPhase(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (resourceRequestPhase, error) {
	if !resourceRequest.GetDeletionTimestamp().IsZero() || !resourceRequest.Spec.WithdrawalTimestamp.IsZero() {
		return deletingResourceRequestPhase, nil
	}

	foreignCluster, err := foreignclusterutils.GetForeignClusterByID(ctx, r.Client, resourceRequest.Spec.ClusterIdentity.ClusterID)
	if err != nil {
		klog.Error(err)
		return denyResourceRequestPhase, err
	}

	if foreignclusterutils.AllowIncomingPeering(foreignCluster, r.Broadcaster.GetConfig()) {
		return allowResourceRequestPhase, nil
	}
	return denyResourceRequestPhase, nil
}
