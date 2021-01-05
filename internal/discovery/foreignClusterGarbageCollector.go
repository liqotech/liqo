package discovery

import (
	goerrors "errors"
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	discoveryPkg "github.com/liqotech/liqo/pkg/discovery"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog"
	"time"
)

func (discovery *DiscoveryCtrl) StartGarbageCollector() {
	for range time.NewTicker(30 * time.Second).C {
		_ = discovery.CollectGarbage()
	}
}

// The GarbageCollector deletes all ForeignClusters discovered with LAN and WAN that have expired TTL
func (discovery *DiscoveryCtrl) CollectGarbage() error {
	req, err := labels.NewRequirement(discoveryPkg.DiscoveryTypeLabel, selection.In, []string{
		string(discoveryPkg.LanDiscovery),
		string(discoveryPkg.WanDiscovery),
	})
	if err != nil {
		klog.Error(err)
		return err
	}
	tmp, err := discovery.crdClient.Resource("foreignclusters").List(metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*req).String(),
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
