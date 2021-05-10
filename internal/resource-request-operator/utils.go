package resourceRequestOperator

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// this function generate an empty offer
func (r *ResourceRequestReconciler) generateResourceOffer(request *discoveryv1alpha1.ResourceRequest) error {
	err := r.computeResources()
	if err != nil {
		return err
	}

	offer := &sharingv1alpha1.ResourceOffer{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.GetNamespace(),
			Name:      offerPrefix + r.ClusterId,
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.Background(), r.Client, offer, func() error {

		offer.Labels = map[string]string{
			discovery.ClusterIdLabel: request.Spec.ClusterIdentity.ClusterID,
		}
		creationTime := metav1.NewTime(time.Now())
		spec := sharingv1alpha1.ResourceOfferSpec{
			ClusterId: r.ClusterId,
			Images:    []corev1.ContainerImage{},
			ResourceQuota: corev1.ResourceQuotaSpec{
				Hard: resources.Offers,
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
	klog.Infof("%s -> %s Offer: %s", r.ClusterId, op, offer.ObjectMeta.Name)
	return nil
}

// this function returns all resource available that will be offered to remote cluster
func (r *ResourceRequestReconciler) computeResources() error {
	//placeholder for future logic
	limits := corev1.ResourceList{}
	limits[corev1.ResourceCPU] = *resource.NewQuantity(2, "2")
	limits[corev1.ResourceMemory] = *resource.NewQuantity(1, "2m")
	resources.Offers = limits
	return nil
}
