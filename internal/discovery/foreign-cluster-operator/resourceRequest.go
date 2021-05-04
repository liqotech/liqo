package foreignclusteroperator

import (
	goerrors "errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator"
)

// createResourceRequest creates a resource request to be sent to the specified ForeignCluster.
func (r *ForeignClusterReconciler) createResourceRequest(
	foreignCluster *discoveryv1alpha1.ForeignCluster) (*discoveryv1alpha1.ResourceRequest, error) {
	localClusterID := r.clusterID.GetClusterID()
	remoteClusterID := foreignCluster.Spec.ClusterIdentity.ClusterID
	localNamespace := foreignCluster.Status.TenantControlNamespace.Local

	// check if a peering request with our cluster id already exists on remote cluster
	tmp, err := r.crdClient.Resource("resourcerequests").Namespace(localNamespace).Get(localClusterID, &metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	resourceRequest, ok := tmp.(*discoveryv1alpha1.ResourceRequest)
	// if resource request does not exists
	if errors.IsNotFound(err) || !ok {
		if _, err = r.namespaceManager.BindClusterRoles(remoteClusterID, r.peeringPermission.Outgoing...); err != nil {
			klog.Error(err)
			return nil, err
		}

		// does not exist -> create new resource request
		authURL, err := r.getHomeAuthURL()
		if err != nil {
			return nil, err
		}
		resourceRequest = &discoveryv1alpha1.ResourceRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: localClusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: fmt.Sprintf(
							"%s/%s",
							discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
						Kind: "ForeignCluster",
						Name: foreignCluster.Name,
						UID:  foreignCluster.UID,
					},
				},
				Labels: map[string]string{
					crdReplicator.LocalLabelSelector: "true",
					crdReplicator.DestinationLabel:   remoteClusterID,
				},
			},
			Spec: discoveryv1alpha1.ResourceRequestSpec{
				ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
					ClusterID:   localClusterID,
					ClusterName: r.ConfigProvider.GetConfig().ClusterName,
				},
				AuthURL: authURL,
			},
		}
		tmp, err = r.crdClient.Resource("resourcerequests").Namespace(
			localNamespace).Create(resourceRequest, &metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		var ok bool
		resourceRequest, ok = tmp.(*discoveryv1alpha1.ResourceRequest)
		if !ok {
			return nil, goerrors.New("created object is not a ResourceRequest")
		}
	}
	// already exists
	return resourceRequest, nil
}

// deleteResourceRequest deletes a resource request related to the specified ForeignCluster.
func (r *ForeignClusterReconciler) deleteResourceRequest(fc *discoveryv1alpha1.ForeignCluster) error {
	localClusterID := r.clusterID.GetClusterID()
	return r.crdClient.Resource("resourcerequests").Namespace(
		fc.Status.TenantControlNamespace.Local).Delete(localClusterID, &metav1.DeleteOptions{})
}
