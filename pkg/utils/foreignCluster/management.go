package foreigncluster

import (
	"strconv"
	"time"

	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// HasHigherPriority upgrades the discovery type. If we discovered a cluster with IncomingPeering, we can upgrade this
// discovery when we found it also in other way, for example inserting a SearchDomain or adding it manually.
func HasHigherPriority(fc *discoveryv1alpha1.ForeignCluster, discoveryType discovery.Type) bool {
	b1 := GetDiscoveryType(fc) == discovery.IncomingPeeringDiscovery
	b2 := discoveryType != discovery.IncomingPeeringDiscovery
	return b1 && b2
}

// LastUpdateNow sets lastUpdate annotation to current time.
func LastUpdateNow(fc *discoveryv1alpha1.ForeignCluster) {
	ann := fc.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[discovery.LastUpdateAnnotation] = strconv.Itoa(int(time.Now().Unix()))
	fc.SetAnnotations(ann)
}

// IsExpired checks if this foreign cluster has been updated before the end of its TimeToLive.
func IsExpired(fc *discoveryv1alpha1.ForeignCluster) bool {
	ann := fc.GetAnnotations()
	if ann == nil {
		return false
	}

	lastUpdate, ok := ann[discovery.LastUpdateAnnotation]
	if !ok {
		return false
	}
	lu, err := strconv.Atoi(lastUpdate)
	if err != nil {
		klog.Error(err)
		return true
	}
	now := time.Now().Unix()
	return int64(lu+fc.Spec.TTL) < now
}
