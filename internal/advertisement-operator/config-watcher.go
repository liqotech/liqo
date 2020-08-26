package advertisement_operator

import (
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	"github.com/liqoTech/liqo/pkg/clusterConfig"
	"github.com/liqoTech/liqo/pkg/crdClient"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"time"
)

func (b *AdvertisementBroadcaster) WatchConfiguration(kubeconfigPath string, client *crdClient.CRDClient) {
	go clusterConfig.WatchConfiguration(func(configuration *policyv1.ClusterConfig) {
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
			return
		}

		if newConfig.ResourceSharingPercentage != b.ClusterConfig.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage {
			// the resource sharing percentage has been modified: update the advertisement
			klog.Infof("AdvertisementConfig changed: the ResourceSharingPercentage has changed from %v to %v",
				b.ClusterConfig.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage, newConfig.ResourceSharingPercentage)
			b.ClusterConfig.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage = newConfig.ResourceSharingPercentage
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
		if newConfig.IngoingConfig != r.ClusterConfig.IngoingConfig {
			// the config update is related to the advertisement operator
			// list all advertisements
			obj, err := r.AdvClient.Resource("advertisements").List(metav1.ListOptions{})
			if err != nil {
				klog.Error(err, "Unable to apply configuration: error listing Advertisements")
				return
			}
			advList := obj.(*protocolv1.AdvertisementList)

			if newConfig.IngoingConfig.AcceptPolicy == policyv1.AutoAcceptWithinMaximum && newConfig.IngoingConfig.MaxAcceptableAdvertisement != r.ClusterConfig.IngoingConfig.MaxAcceptableAdvertisement {
				// the accept policy is set to AutoAcceptWithinMaximum and the Maximum has changed: re-check all Advertisements and update if needed
				klog.Infof("AdvertisementConfig changed: the AcceptPolicy is %v and the MaxAcceptableAdvertisement has changed from %v to %v",
					newConfig.IngoingConfig.AcceptPolicy, configuration.Spec.AdvertisementConfig.IngoingConfig.MaxAcceptableAdvertisement, newConfig.IngoingConfig.MaxAcceptableAdvertisement)
				err, advToUpdate := r.ManageMaximumUpdate(newConfig, advList)
				if err != nil {
					klog.Error(err, err.Error())
					return
				}
				for i := range advToUpdate.Items {
					adv := advList.Items[i]
					r.UpdateAdvertisement(&adv)
				}
			}
		}
	}, client, kubeconfigPath)
}

func (r *AdvertisementReconciler) ManageMaximumUpdate(newConfig policyv1.AdvertisementConfig, advList *protocolv1.AdvertisementList) (error, protocolv1.AdvertisementList) {

	advToUpdate := protocolv1.AdvertisementList{Items: []protocolv1.Advertisement{}}
	if newConfig.IngoingConfig.MaxAcceptableAdvertisement > r.ClusterConfig.IngoingConfig.MaxAcceptableAdvertisement {
		// the maximum has increased: check if there are refused advertisements which now can be accepted
		r.ClusterConfig = newConfig
		for i := 0; i < len(advList.Items); i++ {
			adv := &advList.Items[i]
			if adv.Status.AdvertisementStatus == AdvertisementRefused {
				// the adv was refused: check if now it can be accepted
				r.CheckAdvertisement(adv)
				if adv.Status.AdvertisementStatus == AdvertisementAccepted {
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
