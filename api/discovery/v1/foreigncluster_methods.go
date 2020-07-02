package v1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

func (fc *ForeignCluster) GetConfig(client kubernetes.Interface) (*rest.Config, error) {
	secret, err := client.CoreV1().Secrets(fc.Status.CaDataRef.Namespace).Get(fc.Status.CaDataRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	cnf := rest.Config{
		Host: fc.Spec.ApiUrl,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: secret.Data["caData"],
		},
	}
	cnf.APIPath = "/apis"
	cnf.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	cnf.UserAgent = rest.DefaultKubernetesUserAgent()
	return &cnf, nil
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
	secret, err := client.CoreV1().Secrets(fc.Spec.Namespace).Get("ca-data", metav1.GetOptions{})
	if err != nil {
		return err
	}
	localSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: fc.Name + "-ca-data",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
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
	localSecret, err = localClient.CoreV1().Secrets(localNamespace).Create(localSecret)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		}
		// already exists
		localSecret, err = localClient.CoreV1().Secrets(localNamespace).Get(fc.Name+"-ca-data", metav1.GetOptions{})
		if err != nil {
			return err
		}
	}
	fc.Status.CaDataRef = &v1.ObjectReference{
		Kind:       "Secret",
		Namespace:  localNamespace,
		Name:       localSecret.Name,
		UID:        localSecret.UID,
		APIVersion: "v1",
	}
	return nil
}
