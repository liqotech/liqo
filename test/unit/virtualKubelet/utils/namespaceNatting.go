package utils

import (
	api "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/outgoing"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
	"k8s.io/client-go/kubernetes/fake"
)

type FakeNatter struct {
	namespace map[string]string
}

func (f FakeNatter) NatNamespace(namespace string, create bool) (string, error) {
	return "test", nil
}

func (f FakeNatter) DeNatNamespace(namespace string) (string, error) {
	return "test", nil
}

func NewFakeNatter() namespacesMapping.NamespaceNatter { return FakeNatter{} }

func InitTest(typeRequired string) ri.APIReflector {
	kubeClient := fake.NewSimpleClientset()

	Greflector := &api.GenericAPIReflector{
		Api:              0,
		OutputChan:       nil,
		ForeignClient:    kubeClient,
		NamespaceNatting: NewFakeNatter(),
	}

	if typeRequired == "secrets" {

		reflector := &outgoing.SecretsReflector{
			APIReflector: Greflector,
		}
		reflector.SetSpecializedPreProcessingHandlers()
		return reflector
	} else if typeRequired == "endpointSlices" {
		reflector := &outgoing.EndpointSlicesReflector{
			APIReflector:         Greflector,
			LocalRemappedPodCIDR: types.NewNetworkingOption("localRemappedPodCIDR", "10.0.0.0/16"),
			NodeName:             types.NewNetworkingOption("NodeName", "vk-node"),
		}
		reflector.SetSpecializedPreProcessingHandlers()
		return reflector
	}
	return nil
}
