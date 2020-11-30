package v1alpha1

import (
	"context"
	"crypto/x509"
	goerrors "errors"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
	"strconv"
	"time"
)

func (fc *ForeignCluster) CheckTrusted() (bool, error) {
	cnf := &rest.Config{
		Host: fc.Spec.ApiUrl,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
		},
	}
	client, err := kubernetes.NewForConfig(cnf)
	if err != nil {
		return false, err
	}
	_, err = client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		TimeoutSeconds: pointer.Int64Ptr(1),
	})
	var err509 x509.UnknownAuthorityError
	if errors.IsTimeout(err) {
		// it can be more appropriate to set a different error
		return false, nil
	}
	if err == nil || !goerrors.As(err, &err509) {
		// if I can connect without a x509 error it is trusted
		return true, nil
	}
	return false, nil
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
		_, err := discoveryClient.Resource("foreignclusters").Update(fc.Name, fc, metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return err
		}
	}
	return nil
}

func (fc *ForeignCluster) DeleteAdvertisement(advClient *crdClient.CRDClient) error {
	if fc.Status.Outgoing.Advertisement != nil {
		err := advClient.Resource("advertisements").Delete(fc.Status.Outgoing.Advertisement.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		fc.Status.Outgoing.Advertisement = nil
	}
	return nil
}

// if we discovered a cluster with IncomingPeering we can upgrade this discovery
// when we found it also in other way, for example inserting a SearchDomain or
// adding it manually
func (fc *ForeignCluster) HasHigherPriority(discoveryType DiscoveryType) bool {
	return fc.Spec.DiscoveryType == IncomingPeeringDiscovery && discoveryType != IncomingPeeringDiscovery
}

// sets lastUpdate annotation to current time
func (fc *ForeignCluster) LastUpdateNow() {
	ann := fc.GetAnnotations()
	if ann == nil {
		ann = map[string]string{}
	}
	ann[LastUpdateAnnotation] = strconv.Itoa(int(time.Now().Unix()))
	fc.SetAnnotations(ann)
}

func (fc *ForeignCluster) IsExpired() bool {
	ann := fc.GetAnnotations()
	if ann == nil {
		return false
	}

	lastUpdate, ok := ann[LastUpdateAnnotation]
	if !ok {
		return false
	}
	lu, err := strconv.Atoi(lastUpdate)
	if err != nil {
		klog.Error(err)
		return true
	}
	now := time.Now().Unix()
	return int64(lu+int(fc.Status.Ttl)) < now
}
