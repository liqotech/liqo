package peering_request_operator

import (
	"errors"
	"github.com/liqoTech/liqo/api/discovery/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
)

func (r *PeeringRequestReconciler) UpdateForeignCluster(pr *v1alpha1.PeeringRequest) error {
	tmp, err := r.crdClient.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: "cluster-id=" + pr.Spec.ClusterID,
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
	var cnf *rest.Config
	var err error

	cnf, err = pr.GetConfig(r.crdClient.Client())
	if err != nil {
		if errors.As(err, &v1alpha1.LoadConfigError{}) {
			klog.Warning("using default ForeignConfig")
			cnf = r.ForeignConfig
		} else {
			klog.Error(err, err.Error())
			return nil, err
		}
	}

	fc := &v1alpha1.ForeignCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: pr.Spec.ClusterID,
		},
		Spec: v1alpha1.ForeignClusterSpec{
			ClusterID:        pr.Spec.ClusterID,
			Namespace:        pr.Spec.Namespace,
			Join:             false,
			ApiUrl:           cnf.Host,
			DiscoveryType:    v1alpha1.IncomingPeeringDiscovery,
			AllowUntrustedCA: pr.Spec.OriginClusterSets.AllowUntrustedCA,
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
