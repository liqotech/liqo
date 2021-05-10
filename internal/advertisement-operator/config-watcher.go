package advertisementOperator

import (
	"sync"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterConfig"
	"github.com/liqotech/liqo/pkg/crdClient"
	pkg "github.com/liqotech/liqo/pkg/virtualKubelet"
)

func (b *AdvertisementBroadcaster) WatchConfiguration(kubeconfigPath string, client *crdClient.CRDClient) {
	go clusterConfig.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		newConfig := configuration.Spec.AdvertisementConfig.OutgoingConfig
		if !newConfig.EnableBroadcaster {
			// the broadcaster has been disabled
			klog.Infof("AdvertisementConfig changed: the EnableBroadcaster flag has been set to %v", newConfig.EnableBroadcaster)
			b.ClusterConfig.AdvertisementConfig.OutgoingConfig.EnableBroadcaster = newConfig.EnableBroadcaster
			klog.Info("Stopping sharing resources with cluster " + b.ForeignClusterId)
			err := b.NotifyAdvertisementDeletion()
			if err != nil {
				klog.Errorln(err, "Unable to notify Advertisement deletion to foreign cluster")
			} else {
				// wait for advertisement to be deleted to delete the peering request
				for retry := 0; retry < 3; retry++ {
					advName := pkg.AdvertisementPrefix + b.HomeClusterId
					if _, err := b.RemoteClient.Resource("advertisements").Get(advName, &metav1.GetOptions{}); err != nil && k8serrors.IsNotFound(err) {
						break
					}
					time.Sleep(30 * time.Second)
				}
			}
			// delete the peering request to delete the broadcaster
			if err := b.DiscoveryClient.Resource("peeringrequests").Delete(b.PeeringRequestName, &metav1.DeleteOptions{}); err != nil {
				klog.Error("Unable to delete PeeringRequest " + b.PeeringRequestName)
			}
			return
		}

		if newConfig.ResourceSharingPercentage != b.ClusterConfig.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage {
			// the resource sharing percentage has been modified: update the advertisement
			klog.Infof("AdvertisementConfig changed: the ResourceSharingPercentage has changed from %v to %v",
				b.ClusterConfig.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage, newConfig.ResourceSharingPercentage)
			b.ClusterConfig.AdvertisementConfig.OutgoingConfig = newConfig
			// update Advertisement with new resources (given by the new sharing percentage)
			b.updateAdvertisement()
		}

		if differentLabels(b.ClusterConfig.AdvertisementConfig.LabelPolicies, configuration.Spec.AdvertisementConfig.LabelPolicies) {
			// update label policies
			b.ClusterConfig.AdvertisementConfig.LabelPolicies = configuration.Spec.AdvertisementConfig.LabelPolicies
			b.updateAdvertisement()
		}
	}, client, kubeconfigPath)
}

func (b *AdvertisementBroadcaster) updateAdvertisement() {
	advRes, err := b.GetResourcesForAdv()
	if err != nil {
		klog.Errorln(err, "Error while computing resources for Advertisement")
	}
	advToCreate := b.CreateAdvertisement(advRes)
	_, err = b.SendAdvertisementToForeignCluster(advToCreate)
	if err != nil {
		klog.Errorln(err, "Error while sending Advertisement to cluster "+b.ForeignClusterId)
	}
}

func (r *AdvertisementReconciler) WatchConfiguration(kubeconfigPath string, client *crdClient.CRDClient, wg *sync.WaitGroup) {
	defer wg.Done()
	clusterConfig.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
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

			if newConfig.IngoingConfig.AcceptPolicy == configv1alpha1.AutoAcceptMax && newConfig.IngoingConfig.MaxAcceptableAdvertisement != r.ClusterConfig.IngoingConfig.MaxAcceptableAdvertisement {
				// the accept policy is set to AutoAcceptMax and the Maximum has changed: re-check all Advertisements and update if needed
				klog.Infof("AdvertisementConfig changed: the AcceptPolicy is %v and the MaxAcceptableAdvertisement has changed from %v to %v",
					newConfig.IngoingConfig.AcceptPolicy, r.ClusterConfig.IngoingConfig.MaxAcceptableAdvertisement, newConfig.IngoingConfig.MaxAcceptableAdvertisement)
				err, advToUpdate := r.ManageMaximumUpdate(newConfig, advList)
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

func (r *AdvertisementReconciler) ManageMaximumUpdate(newConfig configv1alpha1.AdvertisementConfig, advList *advtypes.AdvertisementList) (error, advtypes.AdvertisementList) {
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
	return nil, advToUpdate
}

func differentLabels(current []configv1alpha1.LabelPolicy, next []configv1alpha1.LabelPolicy) bool {
	if len(current) != len(next) {
		return true
	}
	for _, l := range current {
		if !contains(next, l) {
			return true
		}
	}
	return false
}

func contains(arr []configv1alpha1.LabelPolicy, el configv1alpha1.LabelPolicy) bool {
	for _, a := range arr {
		if a.Key == el.Key && a.Policy == el.Policy {
			return true
		}
	}
	return false
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
