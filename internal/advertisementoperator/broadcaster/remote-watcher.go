package broadcaster

import (
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
)

func (b *AdvertisementBroadcaster) WatchAdvertisement(homeAdvName string) {
	var err error
	resyncPeriod := 1 * time.Minute

	klog.Info("starting remote advertisement watcher")
	// events are triggered only by modifications on the Advertisement created by the broadcaster on the remote cluster
	// homeClusterAdv is the Advertisement created by home cluster on foreign cluster -> stored remotely
	lo := metav1.ListOptions{
		FieldSelector: "metadata.name=" + homeAdvName,
	}
	ehf := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			b.handlerFunc(obj)
		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			b.handlerFunc(newObj)
		},
		DeleteFunc: func(config interface{}) {
			klog.Info("Adv " + homeAdvName + " has been deleted: stopping the watcher")
			b.RemoteClient.Stop <- struct{}{}
		},
	}

	b.RemoteClient.Store, b.RemoteClient.Stop, err = crdclient.WatchResources(b.RemoteClient,
		"advertisements",
		"",
		resyncPeriod,
		ehf,
		lo)
	if err != nil {
		klog.Error(err)
		return
	}

	klog.Info("correctly created watcher for " + homeAdvName)
}

func (b *AdvertisementBroadcaster) handlerFunc(obj interface{}) {
	homeClusterAdv, ok := obj.(*advtypes.Advertisement)
	if !ok {
		klog.Error("Error casting Advertisement")
		os.Exit(1)
	}
	if homeClusterAdv.Status.AdvertisementStatus != "" {
		err := b.saveAdvStatus(homeClusterAdv)
		if err != nil {
			klog.Error(err)
		} else {
			klog.Info("correctly set peering request status to " + homeClusterAdv.Status.AdvertisementStatus)
		}
	}
}

func (b *AdvertisementBroadcaster) saveAdvStatus(adv *advtypes.Advertisement) error {
	// get the PeeringRequest from the foreign cluster which requested resources
	tmp, err := b.DiscoveryClient.Resource("peeringrequests").Get(b.PeeringRequestName, &metav1.GetOptions{})
	if err != nil {
		return err
	}
	pr := tmp.(*discoveryv1alpha1.PeeringRequest)

	// save the advertisement status (ACCEPTED/REFUSED) in the PeeringRequest
	pr.Status.AdvertisementStatus = adv.Status.AdvertisementStatus
	_, err = b.DiscoveryClient.Resource("peeringrequests").Update(pr.Name, pr, &metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	klog.Infof("PeeringRequest %v status updated with AdvertisementStatus = %v", pr.Name, pr.Status.AdvertisementStatus)
	return nil
}
