package v1alpha1

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	crdclient "github.com/liqotech/liqo/pkg/crdClient"
)

// CreateClient creates a new client for the virtualkubelet.liqo.io group.
func CreateClient(kubeconfig string, configOptions func(config *rest.Config)) (*crdclient.CRDClient, error) {
	var config *rest.Config
	var err error

	if err = AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	config, err = crdclient.NewKubeconfig(kubeconfig, &GroupVersion, configOptions)
	if err != nil {
		panic(err)
	}

	clientSet, err := crdclient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	crdclient.AddToRegistry("namespacenattingtables",
		&NamespaceNattingTable{},
		&NamespaceNattingTableList{},
		Keyer,
		GroupResource)

	return clientSet, nil
}

// Keyer returns a key element to index ClusterConfig CR.
func Keyer(obj runtime.Object) (string, error) {
	ns, ok := obj.(*NamespaceNattingTable)
	if !ok {
		return "", errors.New("cannot cast received object to NamespaceNattingTable")
	}

	return ns.Name, nil
}
