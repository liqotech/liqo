package advertisementoperator

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/utils"
)

// WatchConfiguration watches a ClusterConfig for reconciling updates on ClusterConfig.
func (r *AdvertisementReconciler) WatchConfiguration(kubeconfigPath string, client *crdClient.CRDClient, wg *sync.WaitGroup) {
	defer wg.Done()
	utils.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		newConfig := configuration.Spec.AdvertisementConfig
		if newConfig.IngoingConfig != r.ClusterConfig.IngoingConfig {
			// the config update is related to the advertisement operator
			// list all advertisements
			obj, err := r.AdvClient.Resource("advertisements").List(&metav1.ListOptions{})
			if err != nil {
				klog.Error(err, "Unable to apply configuration: error listing Advertisements")
				return
			}
			advList := obj.(*advtypes.AdvertisementList)

			if newConfig.IngoingConfig.AcceptPolicy == configv1alpha1.AutoAcceptMax &&
				newConfig.IngoingConfig.MaxAcceptableAdvertisement != r.ClusterConfig.IngoingConfig.MaxAcceptableAdvertisement {
				// the accept policy is set to AutoAcceptMax and the Maximum has changed: re-check all Advertisements and update if needed
				klog.Infof("AdvertisementConfig changed: the AcceptPolicy is %v and the MaxAcceptableAdvertisement has changed from %v to %v",
					newConfig.IngoingConfig.AcceptPolicy, r.ClusterConfig.IngoingConfig.MaxAcceptableAdvertisement,
					newConfig.IngoingConfig.MaxAcceptableAdvertisement)
				advToUpdate, err := r.ManageMaximumUpdate(newConfig, advList)
				if err != nil {
					klog.Error(err, err.Error())
					return
				}
				for i := range advToUpdate.Items {
					adv := advToUpdate.Items[i]
					r.UpdateAdvertisement(&adv)
				}
			}
		}
	}, client, kubeconfigPath)
}

func (r *AdvertisementReconciler) ManageMaximumUpdate(newConfig configv1alpha1.AdvertisementConfig, advList *advtypes.AdvertisementList) (advtypes.AdvertisementList, error) {
	advToUpdate := advtypes.AdvertisementList{Items: []advtypes.Advertisement{}}
	if newConfig.IngoingConfig.MaxAcceptableAdvertisement > r.ClusterConfig.IngoingConfig.MaxAcceptableAdvertisement {
		// the maximum has increased: check if there are refused advertisements which now can be accepted
		r.ClusterConfig = newConfig
		for i := 0; i < len(advList.Items); i++ {
			adv := &advList.Items[i]
			if adv.Status.AdvertisementStatus == advtypes.AdvertisementRefused {
				r.CheckAdvertisement(adv)
				if adv.Status.AdvertisementStatus == advtypes.AdvertisementAccepted {
					// the adv status has changed: it must be updated
					advToUpdate.Items = append(advToUpdate.Items, *adv)
				}
			}
		}
	} else {
		// the maximum has decreased: save the new config that will be valid from now on
		// previously accepted adv are not modified
		r.ClusterConfig = newConfig
	}
	return advToUpdate, nil
}

func (r *AdvertisementReconciler) InitCRDClient(kubeconfigPath string) (*crdClient.CRDClient, error) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &configv1alpha1.GroupVersion, nil)
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}

	client, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		return nil, err
	}
	return client, nil
}
