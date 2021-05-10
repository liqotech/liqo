package v1alpha1

import (
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/discovery/utils"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/discovery"
)

func (fc *ForeignCluster) CheckTrusted() (bool, error) {
	_, trustMode, err := utils.GetClusterInfo(fc.Spec.AuthURL)
	return trustMode == discovery.TrustModeTrusted, err
}

func (fc *ForeignCluster) SetAdvertisement(adv *advtypes.Advertisement, discoveryClient *crdClient.CRDClient) error {
	if fc.Status.Outgoing.Advertisement == nil {
		// Advertisement has not been set in ForeignCluster yet
		fc.Status.Outgoing.Advertisement = &v1.ObjectReference{
			Kind:       "Advertisement",
			Name:       adv.Name,
			UID:        adv.UID,
			APIVersion: "sharing.liqo.io/v1alpha1",
		}
		_, err := discoveryClient.Resource("foreignclusters").Update(fc.Name, fc, &metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return err
		}
	}
	return nil
}

func (fc *ForeignCluster) DeleteAdvertisement(advClient *crdClient.CRDClient) error {
	if fc.Status.Outgoing.Advertisement != nil {
		err := advClient.Resource("advertisements").Delete(fc.Status.Outgoing.Advertisement.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		fc.Status.Outgoing.Advertisement = nil
	}
	return nil
}

// HasHigherPriority upgrades the discovery type. If we discovered a cluster with IncomingPeering, we can upgrade this
// discovery when we found it also in other way, for example inserting a SearchDomain or adding it manually.
func (fc *ForeignCluster) HasHigherPriority(discoveryType discovery.Type) bool {
	b1 := fc.Spec.DiscoveryType == discovery.IncomingPeeringDiscovery
	b2 := discoveryType != discovery.IncomingPeeringDiscovery
	return b1 && b2
}

// LastUpdateNow sets lastUpdate annotation to current time.
func (fc *ForeignCluster) LastUpdateNow() {
	ann := fc.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[discovery.LastUpdateAnnotation] = strconv.Itoa(int(time.Now().Unix()))
	fc.SetAnnotations(ann)
}

func (fc *ForeignCluster) IsExpired() bool {
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
	return int64(lu+int(fc.Status.TTL)) < now
}
