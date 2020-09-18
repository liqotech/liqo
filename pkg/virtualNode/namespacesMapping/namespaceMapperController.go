package namespacesMapping

import "github.com/liqotech/liqo/pkg/crdClient"

type NamespaceMapperController struct {
	mapper *NamespaceMapper
}

func NewNamespaceMapperController(client crdClient.NamespacedCRDClientInterface, homeClusterId, foreignClusterId string) *NamespaceMapperController {
	startReflectionChan := make(chan string, 100)
	stopReflectionChan := make(chan string, 100)
	startChan := make(chan struct {}, 100)
	stopChan := make(chan struct {}, 100)

	controller := &NamespaceMapperController{
		mapper: &NamespaceMapper{
			homeClient:       client,
			cache:            namespaceNTCache{},
			homeClusterId:    homeClusterId,
			foreignClusterId: foreignClusterId,
			startReflection:  startReflectionChan,
			stopReflection: stopReflectionChan,
			startMapper:      startChan,
			stopMapper:       stopChan,
		},
	}

	return controller
}

func (c *NamespaceMapperController) PollStartReflection() chan string {
	return c.mapper.startReflection
}

func (c *NamespaceMapperController) PollStopReflection() chan string {
	return c.mapper.stopReflection
}

func (c *NamespaceMapperController) PollStartMapper() chan struct{} {
	return c.mapper.startMapper
}

func (c *NamespaceMapperController) PollEndMapper() chan struct{} {
	return c.mapper.stopMapper
}

func (c *NamespaceMapperController) NatNamespace(namespace string, create bool) (string, error) {
	return c.mapper.NatNamespace(namespace, create)
}

func (c *NamespaceMapperController) DeNatNamespace(namespace string) (string, error) {
	return c.mapper.DeNatNamespace(namespace)
}
