package v1alpha1

import (
	"errors"
	"github.com/liqoTech/liqo/pkg/crdClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// create a client for ClusterConfig CR using a provided kubeconfig
func CreateClusterConfigClient(kubeconfig string) (*crdClient.CRDClient, error) {
	var config *rest.Config
	var err error

	if err = AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	crdClient.AddToRegistry("clusterconfigs", &ClusterConfig{}, &ClusterConfigList{}, Keyer, GroupResource)

	config, err = crdClient.NewKubeconfig(kubeconfig, &GroupVersion)
	if err != nil {
		panic(err)
	}

	clientSet, err := crdClient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	store, stop, err := crdClient.WatchResources(clientSet,
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

	return clientSet, nil
}

func Keyer(obj runtime.Object) (string, error) {
	config, ok := obj.(*ClusterConfig)
	if !ok {
		return "", errors.New("cannot cast received object to ClusterConfig")
	}

	return config.Name, nil
}
