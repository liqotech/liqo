package v1alpha1

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	crdclient "github.com/liqotech/liqo/pkg/crdClient"
)

// CreateClusterConfigClient creates a client for ClusterConfig CR using a provided kubeconfig.
func CreateClusterConfigClient(kubeconfig string, watchResources bool) (*crdclient.CRDClient, error) {
	var config *rest.Config
	var err error

	if err = AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	crdclient.AddToRegistry("clusterconfigs", &ClusterConfig{}, &ClusterConfigList{}, Keyer, ClusterConfigGroupResource)

	config, err = crdclient.NewKubeconfig(kubeconfig, &GroupVersion, nil)
	if err != nil {
		panic(err)
	}

	clientSet, err := crdclient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	if watchResources {
		store, stop, err := crdclient.WatchResources(clientSet,
			"clusterconfigs",
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

// Keyer returns a key element to index ClusterConfig CR.
func Keyer(obj runtime.Object) (string, error) {
	config, ok := obj.(*ClusterConfig)
	if !ok {
		return "", errors.New("cannot cast received object to ClusterConfig")
	}

	return config.Name, nil
}
