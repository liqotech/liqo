package v1alpha1

import (
	"errors"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	crdclient "github.com/liqotech/liqo/pkg/crdClient"
)

// create a client for Advertisement CR using a provided kubeconfig:
// - secret != nil                     : the kubeconfig is extracted from the secret.
// - secret == nil && kubeconfig == "" : use an in-cluster configuration.
// - secret == nil && kubeconfig != "" : read the kubeconfig from the provided filepath.
func CreateAdvertisementClient(kubeconfig string, secret *v1.Secret, watchResources bool,
	configOptions func(config *rest.Config)) (*crdclient.CRDClient, error) {
	var config *rest.Config
	var err error

	if err = AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	crdclient.AddToRegistry("advertisements", &Advertisement{}, &AdvertisementList{}, Keyer, GroupResource)

	if secret == nil {
		config, err = crdclient.NewKubeconfig(kubeconfig, &GroupVersion, configOptions)
		if err != nil {
			panic(err)
		}
	} else {
		config, err = crdclient.NewKubeconfigFromSecret(secret, &GroupVersion)
		if err != nil {
			panic(err)
		}
	}

	clientSet, err := crdclient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	if crdclient.Fake && watchResources {
		store, stop, err := crdclient.WatchResources(clientSet,
			"advertisements",
			"",
			0,
			cache.ResourceEventHandlerFuncs{},
			metav1.ListOptions{})

		if err != nil {
			return nil, err
		}

		clientSet.Store = store
		clientSet.Stop = stop
	}
	return clientSet, nil
}

// Keyer returns a key for the advertisement resource.
func Keyer(obj runtime.Object) (string, error) {
	adv, ok := obj.(*Advertisement)
	if !ok {
		return "", errors.New("cannot cast received object to Advertisement")
	}

	return adv.Name, nil
}
