package v1alpha1

import (
	"errors"
	"github.com/liqotech/liqo/pkg/crdClient"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

//CreateForeignClusterClient creates a client for ForeignCluster CR using a provided kubeconfig.
func CreateForeignClusterClient(kubeconfig string) (*crdClient.CRDClient, error) {
	var config *rest.Config
	var err error
	if err = AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	crdClient.AddToRegistry("foreignclusters", &ForeignCluster{}, &ForeignClusterList{}, ForeignClusterKeyer, ForeignClusterGroupResource)
	config, err = crdClient.NewKubeconfig(kubeconfig, &GroupVersion)
	if err != nil {
		panic(err)
	}
	clientSet, err := crdClient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

func ForeignClusterKeyer(obj runtime.Object) (string, error) {
	fc, ok := obj.(*ForeignCluster)
	if !ok {
		return "", errors.New("cannot cast received object to ForeignCluster")
	}
	return fc.Name, nil
}
