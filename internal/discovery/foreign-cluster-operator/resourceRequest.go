package foreignclusteroperator

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
)

// createResourceRequest creates a resource request to be sent to the specified ForeignCluster.
func (r *ForeignClusterReconciler) createResourceRequest(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (controllerutil.OperationResult, error) {
	klog.Infof("[%v] ensuring ResourceRequest existence", foreignCluster.Spec.ClusterIdentity.ClusterID)

	localClusterID := r.clusterID.GetClusterID()
	remoteClusterID := foreignCluster.Spec.ClusterIdentity.ClusterID
	localNamespace := foreignCluster.Status.TenantNamespace.Local

	authURL, err := r.getHomeAuthURL()
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if _, err = r.namespaceManager.BindClusterRoles(remoteClusterID, r.peeringPermission.Outgoing...); err != nil {
		klog.Error(err)
		return controllerutil.OperationResultNone, err
	}

	resourceRequest := &discoveryv1alpha1.ResourceRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      localClusterID,
			Namespace: localNamespace,
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, resourceRequest, func() error {
		labels := resourceRequest.GetLabels()
		requiredLabels := resourceRequestLabels(remoteClusterID)
		if labels == nil {
			labels = requiredLabels
		} else {
			for k, v := range requiredLabels {
				labels[k] = v
			}
		}
		resourceRequest.SetLabels(labels)

		resourceRequest.Spec = discoveryv1alpha1.ResourceRequestSpec{
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID:   localClusterID,
				ClusterName: r.ConfigProvider.GetConfig().ClusterName,
			},
			AuthURL: authURL,
		}

		return controllerutil.SetControllerReference(foreignCluster, resourceRequest, r.Scheme)
	})
	if err != nil {
		klog.Error(err)
		return controllerutil.OperationResultNone, err
	}
	klog.Infof("[%v] ensured the existence of ResourceRequest (with %v operation)",
		remoteClusterID, result)

	return result, nil
}

// deleteResourceRequest deletes a resource request related to the specified ForeignCluster.
func (r *ForeignClusterReconciler) deleteResourceRequest(ctx context.Context, foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	klog.Infof("[%v] ensuring that the ResourceRequest does not exist", foreignCluster.Spec.ClusterIdentity.ClusterID)
	if err := r.Client.DeleteAllOf(ctx,
		&discoveryv1alpha1.ResourceRequest{}, client.MatchingLabels(resourceRequestLabels(foreignCluster.Spec.ClusterIdentity.ClusterID)),
		client.InNamespace(foreignCluster.Status.TenantNamespace.Local)); err != nil {
		klog.Error(err)
		return err
	}
	klog.Infof("[%v] ensured that the ResourceRequest does not exist", foreignCluster.Spec.ClusterIdentity.ClusterID)
	return nil
}

func resourceRequestLabels(remoteClusterID string) map[string]string {
	return map[string]string{
		crdreplicator.LocalLabelSelector: "true",
		crdreplicator.DestinationLabel:   remoteClusterID,
	}
}
