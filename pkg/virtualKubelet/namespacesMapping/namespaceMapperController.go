package namespacesMapping

import (
	"github.com/liqotech/liqo/pkg/crdClient"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
)

type MapperController interface {
	NamespaceReflectionController
	NamespaceMIrroringController
	NamespaceNatter

	PollStartMapper() chan struct{}
	PollStopMapper() chan struct{}
	ReadyForRestart()
	MappedNamespaces() map[string]string
	WaitForSync()
}

type NamespaceReflectionController interface {
	PollStartOutgoingReflection() chan string
	PollStopOutgoingReflection() chan string
}

type NamespaceMIrroringController interface {
	PollStartIncomingReflection() chan string
	PollStopIncomingReflection() chan string
}

type NamespaceNatter interface {
	NatNamespace(namespace string, create bool) (string, error)
	DeNatNamespace(namespace string) (string, error)
}

type NamespaceMapperController struct {
	mapper *NamespaceMapper
}

func NewNamespaceMapperController(client crdClient.NamespacedCRDClientInterface, foreignClient kubernetes.Interface, homeClusterId, foreignClusterId string) (*NamespaceMapperController, error) {
	controller := &NamespaceMapperController{
		mapper: &NamespaceMapper{
			homeClient: client,
			cache: namespaceNTCache{
				nattingTableName: foreignClusterId,
			},
			foreignClient:           foreignClient,
			homeClusterId:           homeClusterId,
			foreignClusterId:        foreignClusterId,
			startOutgoingReflection: make(chan string, 100),
			startIncomingReflection: make(chan string, 100),
			stopIncomingReflection:  make(chan string, 100),
			stopOutgoingReflection:  make(chan string, 100),
			startMapper:             make(chan struct{}, 100),
			stopMapper:              make(chan struct{}, 100),
			restartReady:            make(chan struct{}, 100),
		},
	}
	if err := controller.mapper.startNattingCache(client); err != nil {
		return nil, err
	}
	if err := controller.mapper.createNattingTable(controller.mapper.foreignClusterId); err != nil {
		klog.Error(err, "cannot initialize namespaceNattingTable")
	}

	return controller, nil
}

func (c *NamespaceMapperController) PollStartOutgoingReflection() chan string {
	return c.mapper.startOutgoingReflection
}

func (c *NamespaceMapperController) PollStartIncomingReflection() chan string {
	return c.mapper.startIncomingReflection
}

func (c *NamespaceMapperController) PollStopOutgoingReflection() chan string {
	return c.mapper.stopOutgoingReflection
}

func (c *NamespaceMapperController) PollStopIncomingReflection() chan string {
	return c.mapper.stopIncomingReflection
}

func (c *NamespaceMapperController) PollStartMapper() chan struct{} {
	return c.mapper.startMapper
}

func (c *NamespaceMapperController) PollStopMapper() chan struct{} {
	return c.mapper.stopMapper
}

func (c *NamespaceMapperController) ReadyForRestart() {
	c.mapper.restartReady <- struct{}{}
}

func (c *NamespaceMapperController) NatNamespace(namespace string, create bool) (string, error) {
	return c.mapper.NatNamespace(namespace, create)
}

func (c *NamespaceMapperController) DeNatNamespace(namespace string) (string, error) {
	return c.mapper.DeNatNamespace(namespace)
}

func (c *NamespaceMapperController) WaitForSync() {
	c.mapper.cache.WaitNamespaceNattingTableSync()
}

func (c *NamespaceMapperController) MappedNamespaces() map[string]string {
	namespaces, err := c.mapper.getMappedNamespaces()
	if err != nil {
		klog.Errorf("error while retrieving natting table - ERR: %v", err)
		return nil
	}

	return namespaces
}
