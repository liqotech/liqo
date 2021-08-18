package resourcerequestoperator

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// ensureForeignCluster ensures the ForeignCluster existence, if not exists we have to add a new one
// with IncomingPeering discovery method.
func (r *ResourceRequestReconciler) ensureForeignCluster(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) (requireSpecUpdate bool, err error) {
	remoteClusterID := resourceRequest.Spec.ClusterIdentity.ClusterID
	// check if a foreignCluster with
	// clusterID == resourceRequest.Spec.ClusterIdentity.ClusterID already exists.
	foreignClusterList := &discoveryv1alpha1.ForeignClusterList{}
	err = r.List(ctx, foreignClusterList, client.MatchingLabels{
		discovery.ClusterIDLabel: remoteClusterID,
	})

	if err != nil {
		klog.Errorf("%s -> unable to List foreignCluster: %s",
			remoteClusterID, err)
		return false, err
	}

	if len(foreignClusterList.Items) != 0 {
		return false, nil
	}

	// if does not exist any ForeignCluster with the required clusterID, create a new one.
	err = r.createForeignCluster(ctx, resourceRequest)
	if err != nil {
		klog.Errorf("%s -> unable to Create foreignCluster: %s", remoteClusterID, err)
		return false, err
	}
	klog.V(3).Infof("foreignCluster %s created", remoteClusterID)

	return true, nil
}

func (r *ResourceRequestReconciler) createForeignCluster(ctx context.Context,
	resourceRequest *discoveryv1alpha1.ResourceRequest) error {
	foreignCluster := &discoveryv1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceRequest.Spec.ClusterIdentity.ClusterID,
			Labels: map[string]string{
				discovery.DiscoveryTypeLabel: string(discovery.IncomingPeeringDiscovery),
				discovery.ClusterIDLabel:     resourceRequest.Spec.ClusterIdentity.ClusterID,
			},
		},
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ClusterIdentity:        resourceRequest.Spec.ClusterIdentity,
			OutgoingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			IncomingPeeringEnabled: discoveryv1alpha1.PeeringEnabledAuto,
			ForeignAuthURL:         resourceRequest.Spec.AuthURL,
			InsecureSkipTLSVerify:  pointer.BoolPtr(true),
		},
	}

	err := r.Client.Create(ctx, foreignCluster)
	if err != nil {
		klog.Error(err)
		return err
	}

	// set the created ForeignCluster as owner of the ResourceRequest to make it able
	// to correctly monitor the incoming peering status.
	err = controllerutil.SetControllerReference(foreignCluster, resourceRequest, r.Scheme)
	if err != nil {
		klog.Error(err)
		return err
	}

	klog.Infof("%s -> Created ForeignCluster %s with IncomingPeering discovery type",
		resourceRequest.Spec.ClusterIdentity.ClusterID, foreignCluster.Name)
	return nil
}

func (r *ResourceRequestReconciler) invalidateResourceOffer(ctx context.Context, request *discoveryv1alpha1.ResourceRequest) error {
	var offer sharingv1alpha1.ResourceOffer
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: request.GetNamespace(),
		Name:      offerPrefix + r.ClusterID,
	}, &offer)
	if apierrors.IsNotFound(err) {
		// ignore not found errors
		return nil
	}
	if err != nil {
		klog.Error(err)
		return err
	}

	switch offer.Status.VirtualKubeletStatus {
	case sharingv1alpha1.VirtualKubeletStatusDeleting, sharingv1alpha1.VirtualKubeletStatusCreated:
		if offer.Spec.WithdrawalTimestamp.IsZero() {
			now := metav1.Now()
			offer.Spec.WithdrawalTimestamp = &now
		}
		err = client.IgnoreNotFound(r.Client.Update(ctx, &offer))
		if err != nil {
			klog.Error(err)
			return err
		}
		klog.Infof("%s -> Offer: %s/%s", r.ClusterID, offer.Namespace, offer.Name)
		return nil
	case sharingv1alpha1.VirtualKubeletStatusNone:
		err = client.IgnoreNotFound(r.Client.Delete(ctx, &offer))
		if err != nil {
			klog.Error(err)
			return err
		}
		if request.Status.OfferWithdrawalTimestamp.IsZero() {
			now := metav1.Now()
			request.Status.OfferWithdrawalTimestamp = &now
		}
		klog.Infof("%s -> Deleted Offer: %s/%s", r.ClusterID, offer.Namespace, offer.Name)
		return nil
	default:
		err := fmt.Errorf("unknown VirtualKubeletStatus %v", offer.Status.VirtualKubeletStatus)
		klog.Error(err)
		return err
	}
}
