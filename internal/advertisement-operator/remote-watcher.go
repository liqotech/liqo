package advertisementOperator

import (
	discoveryv1alpha1 "github.com/liqoTech/liqo/api/discovery/v1alpha1"
	advtypes "github.com/liqoTech/liqo/api/sharing/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
)

func (b *AdvertisementBroadcaster) WatchAdvertisement(homeAdvName, foreignAdvName string) {

	klog.Info("starting remote advertisement watcher")
	watcher, err := b.RemoteClient.Resource("advertisements").Watch(metav1.ListOptions{
		FieldSelector: "metadata.name=" + homeAdvName,
		Watch:         true,
	})
	if err != nil {
		klog.Error(err)
		return
	}
	klog.Info("correctly created watcher for " + homeAdvName)

	// events are triggered only by modifications on the Advertisement created by the broadcaster on the remote cluster
	// homeClusterAdv is the Advertisement created by home cluster on foreign cluster -> stored remotely
	// foreignClusterAdv is the Advertisement created by foreign cluster on home cluster -> stored locally
	for event := range watcher.ResultChan() {
		homeClusterAdv, ok := event.Object.(*advtypes.Advertisement)
		if !ok {
			klog.Error("Received object is not an Advertisement")
			continue
		}
		switch event.Type {
		case watch.Added, watch.Modified:
			// the triggering event is a modification made by the tunnelEndpoint creator
			if homeClusterAdv.Status.RemoteRemappedPodCIDR != "" {
				err = b.setNetworkRemapping(homeClusterAdv, foreignAdvName)
				if err != nil {
					klog.Error(err)
				} else {
					klog.Info("correctly set network remapping for foreign advertisement " + foreignAdvName)
				}
			}

			// the triggering event is the acceptance/refusal of the Advertisement
			if homeClusterAdv.Status.AdvertisementStatus != "" {
				err = b.saveAdvStatus(homeClusterAdv)
				if err != nil {
					klog.Error(err)
				} else {
					klog.Info("correctly set peering request status to " + homeClusterAdv.Status.AdvertisementStatus)
				}
			}
		case watch.Deleted:
			klog.Info("Adv " + homeAdvName + " has been deleted")
			watcher.Stop()
		}
	}
}

func (b *AdvertisementBroadcaster) setNetworkRemapping(homeClusterAdv *advtypes.Advertisement, foreignAdvName string) error {
	// get the Advertisement of the foreign cluster (stored in the local cluster)
	obj, err := b.LocalClient.Resource("advertisements").Get(foreignAdvName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	foreignClusterAdv := obj.(*advtypes.Advertisement)
	// set the status of the foreign cluster Advertisement with the information given by the tunnelEndpoint creator
	foreignClusterAdv.Status.LocalRemappedPodCIDR = homeClusterAdv.Status.RemoteRemappedPodCIDR
	_, err = b.LocalClient.Resource("advertisements").UpdateStatus(foreignAdvName, foreignClusterAdv, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (b *AdvertisementBroadcaster) saveAdvStatus(adv *advtypes.Advertisement) error {
	// get the PeeringRequest from the foreign cluster which requested resources
	tmp, err := b.DiscoveryClient.Resource("peeringrequests").Get(b.PeeringRequestName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	pr := tmp.(*discoveryv1alpha1.PeeringRequest)

	// save the advertisement status (ACCEPTED/REFUSED) in the PeeringRequest
	pr.Status.AdvertisementStatus = adv.Status.AdvertisementStatus
	_, err = b.DiscoveryClient.Resource("peeringrequests").UpdateStatus(b.PeeringRequestName, pr, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
