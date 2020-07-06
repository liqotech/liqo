package advertisement_operator

import (
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	"github.com/liqoTech/liqo/pkg/clusterConfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func (b *AdvertisementBroadcaster) WatchConfiguration(kubeconfigPath string) {
	go clusterConfig.WatchConfiguration(func(configuration *policyv1.ClusterConfig) {
		b.ClusterConfig.ResourceSharingPercentage = configuration.Spec.AdvertisementConfig.ResourceSharingPercentage
	}, nil, kubeconfigPath)
}

func (r *AdvertisementReconciler) WatchConfiguration(kubeconfigPath string) {
	go clusterConfig.WatchConfiguration(func(configuration *policyv1.ClusterConfig) {
		obj, err := r.AdvClient.Resource("advertisements").List(metav1.ListOptions{})
		if err != nil {
			klog.Error(err, "Unable to apply configuration: error listing Advertisements")
			return
		}
		advList := obj.(*protocolv1.AdvertisementList)
		err, updateFlag := r.ManageConfigUpdate(configuration, advList)
		if err != nil {
			klog.Error(err, err.Error())
			return
		}
		if updateFlag {
			for _, adv := range advList.Items {
				r.UpdateAdvertisement(&adv)
			}
		}
	}, nil, kubeconfigPath)
}

func (r *AdvertisementReconciler) ManageConfigUpdate(configuration *policyv1.ClusterConfig, advList *protocolv1.AdvertisementList) (error, bool) {

	updateFlag := false
	if configuration.Spec.AdvertisementConfig.MaxAcceptableAdvertisement > r.ClusterConfig.MaxAcceptableAdvertisement {
		// the maximum has increased: check if there are refused advertisements which now can be accepted
		r.ClusterConfig = configuration.Spec.AdvertisementConfig
		for i := 0; i < len(advList.Items); i++ {
			adv := &advList.Items[i]
			if adv.Status.AdvertisementStatus == "REFUSED" {
				r.CheckAdvertisement(adv)
				updateFlag = true
			}
		}
	} else {
		// the maximum has decreased: if the already accepted advertisements are too many (with the new maximum), delete some of them
		r.ClusterConfig = configuration.Spec.AdvertisementConfig
		if r.ClusterConfig.MaxAcceptableAdvertisement < r.AcceptedAdvNum {
			for i := 0; i < int(r.AcceptedAdvNum-r.ClusterConfig.MaxAcceptableAdvertisement); i++ {
				adv := advList.Items[i]
				if adv.Status.AdvertisementStatus == "ACCEPTED" {
					err := r.AdvClient.Resource("advertisements").Delete(adv.Name, metav1.DeleteOptions{})
					if err != nil {
						klog.Errorln(err, "Unable to apply configuration: error deleting Advertisement "+adv.Name)
						return err, updateFlag
					}
					r.AcceptedAdvNum--
				}
			}
		}
	}
	return nil, updateFlag
}
