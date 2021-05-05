package resourcerequestoperator

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// generateResourceOffer generates a new local ResourceOffer.
func (r *ResourceRequestReconciler) generateResourceOffer(request *discoveryv1alpha1.ResourceRequest) error {
	resources, err := r.Broadcaster.ReadResources()
	if err != nil {
		return err
	}

	offer := &sharingv1alpha1.ResourceOffer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.GetNamespace(),
			Name:      offerPrefix + r.ClusterID,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.Background(), r.Client, offer, func() error {
		offer.Labels = map[string]string{
			discovery.ClusterIDLabel: request.Spec.ClusterIdentity.ClusterID,
		}
		creationTime := metav1.NewTime(time.Now())
		spec := sharingv1alpha1.ResourceOfferSpec{
			ClusterId: r.ClusterID,
			Images:    []corev1.ContainerImage{},
			ResourceQuota: corev1.ResourceQuotaSpec{
				Hard: resources,
			},
			Timestamp:  creationTime,
			TimeToLive: metav1.NewTime(creationTime.Add(timeToLive)),
		}
		offer.Spec = spec
		return controllerutil.SetOwnerReference(request, offer, r.Scheme)
	})

	if err != nil {
		return err
	}
	klog.Infof("%s -> %s Offer: %s", r.ClusterID, op, offer.ObjectMeta.Name)
	return nil
}
