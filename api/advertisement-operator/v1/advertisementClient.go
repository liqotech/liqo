package v1

import (
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// create a client for Advertisement CR using a provided kubeconfig
// - secret != nil                     : the kubeconfig is extracted from the secret
// - secret == nil && kubeconfig == "" : use an in-cluster configuration
// - secret == nil && kubeconfig != "" : read the kubeconfig from the provided filepath
func CreateAdvertisementClient(kubeconfig string, secret *v1.Secret) (*v1alpha1.CRDClient, error) {
	var config *rest.Config
	var err error

	if err = AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	if secret == nil {
		config, err = v1alpha1.NewKubeconfig(kubeconfig, &GroupVersion)
		if err != nil {
			panic(err)
		}
	} else {
		config, err = v1alpha1.NewKubeconfigFromSecret(secret, &GroupVersion)
		if err != nil {
			panic(err)
		}
	}
	clientSet, err := v1alpha1.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	v1alpha1.AddToRegistry("advertisements", &Advertisement{}, &AdvertisementList{}, nil, GroupResource)

	return clientSet, nil
}
