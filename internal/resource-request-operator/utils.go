package resourcerequestoperator

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/discovery"
)

// generateResourceOffer generates a new local ResourceOffer.
func (r *ResourceRequestReconciler) generateResourceOffer(ctx context.Context, request *discoveryv1alpha1.ResourceRequest) error {
	resources := r.Broadcaster.ReadResources(request.Spec.ClusterIdentity.ClusterID)
	offer := &sharingv1alpha1.ResourceOffer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.GetNamespace(),
			Name:      offerPrefix + r.ClusterID,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, offer, func() error {
		offer.Labels = map[string]string{
			discovery.ClusterIDLabel:         request.Spec.ClusterIdentity.ClusterID,
			crdreplicator.LocalLabelSelector: "true",
			crdreplicator.DestinationLabel:   request.Spec.ClusterIdentity.ClusterID,
		}
		creationTime := metav1.NewTime(time.Now())
		spec := sharingv1alpha1.ResourceOfferSpec{
			ClusterId: r.ClusterID,
			Images:    []corev1.ContainerImage{},
			ResourceQuota: corev1.ResourceQuotaSpec{
				Hard: resources,
			},
			Labels:     r.Broadcaster.clusterConfig.Spec.DiscoveryConfig.ClusterLabels,
			Timestamp:  creationTime,
			TimeToLive: metav1.NewTime(creationTime.Add(timeToLive)),
		}
		offer.Spec = spec
		return controllerutil.SetControllerReference(request, offer, r.Scheme)
	})

	if err != nil {
		klog.Error(err)
		return err
	}
	klog.Infof("%s -> %s Offer: %s/%s", r.ClusterID, op, offer.Namespace, offer.Name)
	return nil
}

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
			ClusterIdentity: resourceRequest.Spec.ClusterIdentity,
			Namespace:       resourceRequest.Namespace,
			Join:            false,
			DiscoveryType:   discovery.IncomingPeeringDiscovery,
			AuthURL:         resourceRequest.Spec.AuthURL,
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
