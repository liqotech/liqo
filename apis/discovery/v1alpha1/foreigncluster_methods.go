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
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
)

func (fc *ForeignCluster) GetConfig(client kubernetes.Interface) (*rest.Config, error) {
	var cnf rest.Config
	if fc.Status.TrustMode == TrustModeTrusted {
		// ForeignCluster uses a trusted CA, it doesn't require to load retrieved CA
		cnf = rest.Config{
			Host: fc.Spec.ApiUrl,
		}
	} else {
		// load retrieved CA
		secret, err := client.CoreV1().Secrets(fc.Status.Outgoing.CaDataRef.Namespace).Get(context.TODO(), fc.Status.Outgoing.CaDataRef.Name, metav1.GetOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return nil, err
		}
		cnf = rest.Config{
			Host: fc.Spec.ApiUrl,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: secret.Data["caData"],
			},
		}
	}
	cnf.APIPath = "/apis"
	cnf.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	cnf.UserAgent = rest.DefaultKubernetesUserAgent()
	return &cnf, nil
}

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

func (fc *ForeignCluster) getInsecureConfig() *rest.Config {
	cnf := rest.Config{
		Host: fc.Spec.ApiUrl,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
	return &cnf
}

func (fc *ForeignCluster) LoadForeignCA(localClient kubernetes.Interface, localNamespace string, config *rest.Config) error {
	if config == nil {
		config = fc.getInsecureConfig()
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	secret, err := client.CoreV1().Secrets(fc.Spec.Namespace).Get(context.TODO(), "ca-data", metav1.GetOptions{})
	if err != nil {
		return err
	}
	localSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: fc.Name + "-ca-data",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1alpha1",
					Kind:       "ForeignCluster",
					Name:       fc.Name,
					UID:        fc.UID,
				},
			},
		},
		Data: map[string][]byte{
			"caData": secret.Data["ca.crt"],
		},
	}
	localSecret, err = localClient.CoreV1().Secrets(localNamespace).Create(context.TODO(), localSecret, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		// already exists
		localSecret, err = localClient.CoreV1().Secrets(localNamespace).Get(context.TODO(), fc.Name+"-ca-data", metav1.GetOptions{})
		if err != nil {
			return err
		}
	}
	fc.Status.Outgoing.CaDataRef = &v1.ObjectReference{
		Kind:       "Secret",
		Namespace:  localNamespace,
		Name:       localSecret.Name,
		UID:        localSecret.UID,
		APIVersion: "v1",
	}
	return nil
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
		if err != nil {
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
