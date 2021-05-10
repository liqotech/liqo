package broadcaster

import (
	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/utils"
	pkg "github.com/liqotech/liqo/pkg/virtualKubelet"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"time"
)

func (b *AdvertisementBroadcaster) WatchConfiguration(kubeconfigPath string, client *crdClient.CRDClient) {
	go utils.WatchConfiguration(func(configuration *configv1alpha1.ClusterConfig) {
		newConfig := configuration.Spec.AdvertisementConfig.OutgoingConfig
		if !newConfig.EnableBroadcaster {
			// the broadcaster has been disabled
			klog.Infof("AdvertisementConfig changed: the EnableBroadcaster flag has been set to %v", newConfig.EnableBroadcaster)
			b.ClusterConfig.AdvertisementConfig.OutgoingConfig.EnableBroadcaster = newConfig.EnableBroadcaster
			klog.Info("Stopping sharing resources with cluster " + b.ForeignClusterID)
			err := b.NotifyAdvertisementDeletion()
			if err != nil {
				klog.Errorln(err, "Unable to notify Advertisement deletion to foreign cluster")
			} else {
				// wait for advertisement to be deleted to delete the peering request
				for retry := 0; retry < 3; retry++ {
					advName := pkg.AdvertisementPrefix + b.HomeClusterID
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
		klog.Errorln(err, "Error while sending Advertisement to cluster "+b.ForeignClusterID)
	}
}
