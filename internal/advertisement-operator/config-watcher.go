package advertisement_operator

import (
	"context"
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	"github.com/liqoTech/liqo/pkg/clusterConfig"
	"github.com/liqoTech/liqo/pkg/crdClient"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func (b *AdvertisementBroadcaster) WatchConfiguration(kubeconfigPath string, client *crdClient.CRDClient) {
	go clusterConfig.WatchConfiguration(func(configuration *policyv1.ClusterConfig) {
		newConfig := configuration.Spec.AdvertisementConfig.BroadcasterConfig
		if !newConfig.EnableBroadcaster {
			// the broadcaster has been disabled
			klog.Infof("AdvertisementConfig changed: the EnableBroadcaster flag has been set to %v", newConfig.EnableBroadcaster)
			b.ClusterConfig.AdvertisementConfig.EnableBroadcaster = newConfig.EnableBroadcaster
			klog.Info("Stopping sharing resources with cluster " + b.ForeignClusterId)
			err := b.NotifyAdvertisementDeletion()
			if err != nil {
				klog.Errorln(err, "Unable to notify Advertisement deletion to foreign cluster")
			} else {
				// wait for advertisement to be deleted to delete the peering request
				for retry := 0; retry < 3; retry++ {
					advName := "advertisement-" + b.HomeClusterId
					if _, err := b.RemoteClient.Resource("advertisements").Get(advName, metav1.GetOptions{}); err != nil && k8serrors.IsNotFound(err) {
						break
					}
					time.Sleep(30 * time.Second)
				}
			}
			// delete the peering request to delete the broadcaster
			if err := b.DiscoveryClient.Resource("peeringrequests").Delete(b.PeeringRequestName, metav1.DeleteOptions{}); err != nil {
				klog.Error("Unable to delete PeeringRequest " + b.PeeringRequestName)
			}
		}

		if newConfig.ResourceSharingPercentage != b.ClusterConfig.AdvertisementConfig.ResourceSharingPercentage {
			// the resource sharing percentage has been modified: update the advertisement
			klog.Infof("AdvertisementConfig changed: the ResourceSharingPercentage has changed from %v to %v",
				b.ClusterConfig.AdvertisementConfig.ResourceSharingPercentage, newConfig.ResourceSharingPercentage)
			b.ClusterConfig.AdvertisementConfig.ResourceSharingPercentage = newConfig.ResourceSharingPercentage
			// update Advertisement with new resources (given by the new sharing percentage)
			physicalNodes, virtualNodes, availability, limits, images, err := b.GetResourcesForAdv()
			if err != nil {
				klog.Errorln(err, "Error while computing resources for Advertisement")
			}
			advToCreate := b.CreateAdvertisement(physicalNodes, virtualNodes, availability, images, limits)
			_, err = b.SendAdvertisementToForeignCluster(advToCreate)
			if err != nil {
				klog.Errorln(err, "Error while sending Advertisement to cluster "+b.ForeignClusterId)
			}
		}

	}, client, kubeconfigPath)
}

func (r *AdvertisementReconciler) WatchConfiguration(kubeconfigPath string, client *crdClient.CRDClient) {
	go clusterConfig.WatchConfiguration(func(configuration *policyv1.ClusterConfig) {
		newConfig := configuration.Spec.AdvertisementConfig
		if newConfig.AdvOperatorConfig != r.ClusterConfig.AdvOperatorConfig {
			// the config update is related to the advertisement operator
			// list all advertisements
			obj, err := r.AdvClient.Resource("advertisements").List(metav1.ListOptions{})
			if err != nil {
				klog.Error(err, "Unable to apply configuration: error listing Advertisements")
				return
			}
			advList := obj.(*protocolv1.AdvertisementList)

			// check the type of update
			if newConfig.AcceptPolicy != r.ClusterConfig.AcceptPolicy {
				// the AcceptPolicy has changed: all Advertisements need to be checked again according to the new policy
				klog.Infof("AdvertisementConfig changed: the AcceptPolicy has changed from %v to %v", r.ClusterConfig.AcceptPolicy, newConfig.AcceptPolicy)
				if r.ClusterConfig.AcceptPolicy == policyv1.AutoAcceptAll {
					// if the previous policy was AutoAcceptAll, the MaxAcceptableAdvertisement field has been ignored
					// therefore, we could have more accepted advertisements than the maximum
					// check the maximum is respected
					err, _ := r.ManageMaximumUpdate(newConfig, advList)
					if err != nil {
						klog.Error(err, err.Error())
						return
					}
				}
				// update the config saved in the reconciler
				r.ClusterConfig = newConfig
				// check all advertisements with the new policy
				for i := range advList.Items {
					adv := advList.Items[i]
					r.CheckAdvertisement(&adv)
					r.UpdateAdvertisement(&adv)
				}
			}
			if newConfig.AcceptPolicy == policyv1.AutoAcceptWithinMaximum && newConfig.MaxAcceptableAdvertisement != r.ClusterConfig.MaxAcceptableAdvertisement {
				// the accept policy is set to AutoAcceptWithinMaximum and the Maximum has changed: re-check all Advertisements and update if needed
				klog.Infof("AdvertisementConfig changed: the AcceptPolicy is %v and the MaxAcceptableAdvertisement has changed from %v to %v",
					newConfig.AcceptPolicy, configuration.Spec.AdvertisementConfig.MaxAcceptableAdvertisement, newConfig.MaxAcceptableAdvertisement)
				err, updateFlag := r.ManageMaximumUpdate(newConfig, advList)
				if err != nil {
					klog.Error(err, err.Error())
					return
				}
				if updateFlag {
					for i := range advList.Items {
						adv := advList.Items[i]
						r.UpdateAdvertisement(&adv)
					}
				}
			}
		}
	}, client, kubeconfigPath)
}

func (r *AdvertisementReconciler) ManageMaximumUpdate(newConfig policyv1.AdvertisementConfig, advList *protocolv1.AdvertisementList) (error, bool) {

	updateFlag := false
	if newConfig.MaxAcceptableAdvertisement > r.ClusterConfig.MaxAcceptableAdvertisement {
		// the maximum has increased: check if there are refused advertisements which now can be accepted
		r.ClusterConfig = newConfig
		for i := 0; i < len(advList.Items); i++ {
			adv := &advList.Items[i]
			if adv.Status.AdvertisementStatus == AdvertisementRefused {
				r.CheckAdvertisement(adv)
				updateFlag = true
			}
		}
	} else {
		// the maximum has decreased: if the already accepted advertisements are too many (with the new maximum), delete some of them
		r.ClusterConfig = newConfig
		if r.ClusterConfig.MaxAcceptableAdvertisement < r.AcceptedAdvNum {
			advToDelete := int(r.AcceptedAdvNum - r.ClusterConfig.MaxAcceptableAdvertisement)
			for i := 0; i < advToDelete; i++ {
				adv := advList.Items[i]
				if adv.Status.AdvertisementStatus == AdvertisementAccepted {
					err := r.Delete(context.Background(), &adv, &client.DeleteOptions{})
					if err != nil {
						klog.Errorln(err, "Unable to apply new configuration: error deleting Advertisement "+adv.Name)
						return err, updateFlag
					}
					r.AcceptedAdvNum--
				}
			}
		}
	}
	return nil, updateFlag
}
