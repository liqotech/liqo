package v1alpha1

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/crdClient"
)

func CreateClient(kubeconfig string, configOptions func(config *rest.Config)) (*crdClient.CRDClient, error) {
	var config *rest.Config
	var err error

	if err = AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}

	config, err = crdClient.NewKubeconfig(kubeconfig, &GroupVersion, configOptions)
	if err != nil {
		panic(err)
	}

	clientSet, err := crdClient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}

	crdClient.AddToRegistry("namespacenattingtables",
		&NamespaceNattingTable{},
		&NamespaceNattingTableList{},
		Keyer,
		GroupResource)

	return clientSet, nil
}

func Keyer(obj runtime.Object) (string, error) {
	ns, ok := obj.(*NamespaceNattingTable)
	if !ok {
		return "", errors.New("cannot cast received object to NamespaceNattingTable")
	}

	return ns.Name, nil
}
