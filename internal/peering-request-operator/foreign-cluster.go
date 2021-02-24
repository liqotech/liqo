package peering_request_operator

import (
	"errors"
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
	"strings"
)

func (r *PeeringRequestReconciler) UpdateForeignCluster(pr *v1alpha1.PeeringRequest) error {
	tmp, err := r.crdClient.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: strings.Join([]string{
			discovery.ClusterIdLabel,
			pr.Spec.ClusterIdentity.ClusterID,
		}, "="),
	})
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	fcList, ok := tmp.(*v1alpha1.ForeignClusterList)
	if !ok {
		err = errors.New("retrieved object is not a ForeignClusterList")
		klog.Error(err, err.Error())
		return err
	}

	if len(fcList.Items) == 0 {
		// create it
		fc, err := r.createForeignCluster(pr)
		if err != nil {
			return err
		}
		r.setOwner(pr, fc)
		return nil
	} else {
		// update it
		fc := &fcList.Items[0]
		if fc.Status.Incoming.PeeringRequest != nil {
			// already up to date
			return nil
		}
		fc.Status.Incoming.PeeringRequest = &corev1.ObjectReference{
			Kind:       pr.Kind,
			Name:       pr.Name,
			UID:        pr.UID,
			APIVersion: pr.APIVersion,
		}
		_, err = r.crdClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return err
		}
		r.setOwner(pr, fc)
		return nil
	}
}

func (r *PeeringRequestReconciler) createForeignCluster(pr *v1alpha1.PeeringRequest) (*v1alpha1.ForeignCluster, error) {
	var err error

	fc := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: pr.Spec.ClusterIdentity.ClusterID,
			Labels: map[string]string{
				discovery.DiscoveryTypeLabel: string(discovery.IncomingPeeringDiscovery),
				discovery.ClusterIdLabel:     pr.Spec.ClusterIdentity.ClusterID,
			},
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterIdentity: pr.Spec.ClusterIdentity,
			Namespace:       pr.Spec.Namespace,
			Join:            false,
			DiscoveryType:   discovery.IncomingPeeringDiscovery,
			AuthUrl:         pr.Spec.AuthUrl,
		},
		Status: v1alpha1.ForeignClusterStatus{
			Incoming: v1alpha1.Incoming{
				PeeringRequest: &corev1.ObjectReference{
					Kind:       pr.Kind,
					Name:       pr.Name,
					UID:        pr.UID,
					APIVersion: pr.APIVersion,
				},
			},
		},
	}

	tmp, err := r.crdClient.Resource("foreignclusters").Create(fc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}
	fc, ok := tmp.(*v1alpha1.ForeignCluster)
	if !ok {
		err = errors.New("retrieved object is not a ForeignCluster")
		klog.Error(err, err.Error())
		return nil, err
	}
	return fc, nil
}

func (r *PeeringRequestReconciler) setOwner(pr *v1alpha1.PeeringRequest, fc *v1alpha1.ForeignCluster) {
	if pr.OwnerReferences == nil {
		pr.OwnerReferences = []metav1.OwnerReference{}
	}
	pr.OwnerReferences = append(pr.OwnerReferences, metav1.OwnerReference{
		APIVersion: "discovery.liqo.io/v1alpha1",
		Kind:       "ForeignCluster",
		Name:       fc.Name,
		UID:        fc.UID,
		Controller: pointer.BoolPtr(true),
	})
}
