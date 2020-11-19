package discovery

import (
	goerrors "errors"
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"strings"
	"time"
)

func (discovery *DiscoveryCtrl) StartGarbageCollector() {
	for range time.NewTicker(30 * time.Second).C {
		_ = discovery.CollectGarbage()
	}
}

// The GarbageCollector deletes all ForeignClusters discovered with LAN that have expired TTL
func (discovery *DiscoveryCtrl) CollectGarbage() error {
	tmp, err := discovery.crdClient.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: strings.Join([]string{"discovery-type", string(v1alpha1.LanDiscovery)}, "="),
	})
	if err != nil {
		klog.Error(err)
		return err
	}
	fcs, ok := tmp.(*v1alpha1.ForeignClusterList)
	if !ok {
		err = goerrors.New("retrieved object is not a ForeignCluster")
		klog.Error(err)
		return err
	}

	for _, fc := range fcs.Items {
		if fc.IsExpired() {
			klog.V(4).Infof("delete foreignCluster %v (TTL expired)", fc.Name)
			err = discovery.crdClient.Resource("foreignclusters").Delete(fc.Name, metav1.DeleteOptions{})
			if err != nil {
				klog.Error(err)
				continue
			}
		}
	}
	return nil
}
