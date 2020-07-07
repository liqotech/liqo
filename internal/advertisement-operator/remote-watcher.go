package advertisement_operator

import (
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	"github.com/liqoTech/liqo/pkg/crdClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
)

func WatchAdvertisement(localClient, remoteClient *crdClient.CRDClient, homeAdvName, foreignAdvName string) {

	klog.V(6).Info("starting remote advertisement watcher")
	watcher, err := remoteClient.Resource("advertisements").Watch(metav1.ListOptions{
		FieldSelector: "metadata.name=" + homeAdvName,
		Watch:         true,
	})
	if err != nil {
		klog.Error(err)
		return
	}
	klog.V(6).Info("correctly created watcher for " + homeAdvName)

	// events are triggered only by modifications on the Advertisement created by the broadcaster on the remote cluster
	// homeClusterAdv is the Advertisement created by home cluster on foreign cluster -> stored remotely
	// foreignClusterAdv is the Advertisement created by foreign cluster on home cluster -> stored locally
	for event := range watcher.ResultChan() {
		homeClusterAdv, ok := event.Object.(*protocolv1.Advertisement)
		if !ok {
			klog.Error("Received object is not an Advertisement")
			continue
		}
		switch event.Type {
		case watch.Added, watch.Modified:
			// check if the triggering event is a modification made by the tunnelEndpoint creator
			if homeClusterAdv.Status.RemoteRemappedPodCIDR == "" {
				continue
			}

			// get the Advertisement of the foreign cluster (stored in the local cluster)
			obj, err := localClient.Resource("advertisements").Get(foreignAdvName, metav1.GetOptions{})
			if err != nil {
				klog.Error(err)
				continue
			}
			foreignClusterAdv := obj.(*protocolv1.Advertisement)
			// set the status of the foreign cluster Advertisement with the information given by the tunnelEndpoint creator
			foreignClusterAdv.Status.LocalRemappedPodCIDR = homeClusterAdv.Status.RemoteRemappedPodCIDR
			_, err = localClient.Resource("advertisements").UpdateStatus(foreignAdvName, foreignClusterAdv, metav1.UpdateOptions{})
			if err != nil {
				klog.Error(err)
				continue
			}
			klog.V(6).Info("correctly set status of foreign advertisement " + foreignAdvName)
		case watch.Deleted:
			klog.Info("Adv " + homeAdvName + " has been deleted")
		}
	}
}
