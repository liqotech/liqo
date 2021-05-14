package v1alpha1

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	crdclient "github.com/liqotech/liqo/pkg/crdClient"
)

// CreateForeignClusterClient creates a client for ForeignCluster CR using a provided kubeconfig.
func CreateForeignClusterClient(kubeconfig string) (*crdclient.CRDClient, error) {
	var config *rest.Config
	var err error
	if err = AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	crdclient.AddToRegistry("foreignclusters", &ForeignCluster{},
		&ForeignClusterList{}, ForeignClusterKeyer, ForeignClusterGroupResource)
	config, err = crdclient.NewKubeconfig(kubeconfig, &GroupVersion, nil)
	if err != nil {
		panic(err)
	}
	clientSet, err := crdclient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

// ForeignClusterKeyer returns a key element to index ForeignCluster CR.
func ForeignClusterKeyer(obj runtime.Object) (string, error) {
	fc, ok := obj.(*ForeignCluster)
	if !ok {
		return "", errors.New("cannot cast received object to ForeignCluster")
	}
	return fc.Name, nil
}
