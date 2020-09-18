package apiReflection

import (
	reflectedApi "github.com/liqotech/liqo/pkg/virtualNode/apiReflection/api"
	"github.com/liqotech/liqo/pkg/virtualNode/namespacesMapping"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sync"
	"time"
)

var defaultResyncPeriod = 10 * time.Second

type APIReflectorController struct {
	outputChan        chan reflectedApi.ApiEvent
	homeClient        kubernetes.Interface
	foreignClient     kubernetes.Interface
	informerFactories map[string]informers.SharedInformerFactory
	apiReflectors     map[reflectedApi.ApiType]reflectedApi.APIReflector
	waitGroup         *sync.WaitGroup
	namespaceNatting  namespacesMapping.NamespaceController
	stop              chan struct{}
}

func NewAPIReflectorController(homeClient, foreignClient kubernetes.Interface,
	outputChan chan reflectedApi.ApiEvent,
	namespaceNatting namespacesMapping.NamespaceController) *APIReflectorController {
	controller := &APIReflectorController{
		outputChan:        outputChan,
		homeClient:        homeClient,
		foreignClient:     foreignClient,
		informerFactories: make(map[string]informers.SharedInformerFactory),
		apiReflectors:     make(map[reflectedApi.ApiType]reflectedApi.APIReflector),
		namespaceNatting:  namespaceNatting,
		stop:              make(chan struct{}),
	}

	for api := range reflectedApi.ApiMapping {
		controller.buildApiReflector(api)
	}

	return controller
}

func (c *APIReflectorController) Start() {
	for {
		select {
		case ns := <-c.namespaceNatting.PollStartReflection():
			c.reflectNamespace(ns)
		}
	}
}

func (c *APIReflectorController) buildApiReflector(api reflectedApi.ApiType) reflectedApi.APIReflector {
	apiReflector := &reflectedApi.GenericAPIReflector{
		Api:              api,
		OutputChan:       c.outputChan,
		ForeignClient:    c.foreignClient,
		Informers:        make(map[string]cache.SharedIndexInformer),
		NamespaceNatting: c.namespaceNatting,
	}
	return reflectedApi.ApiMapping[api](apiReflector)
}

func (c *APIReflectorController) reflectNamespace(namespace string) {

	factory := informers.NewSharedInformerFactoryWithOptions(c.homeClient, defaultResyncPeriod, informers.WithNamespace(namespace))
	c.informerFactories[namespace] = factory

	for api, handler := range reflectedApi.InformerBuilding {
		informer := handler(factory)
		c.apiReflectors[api].ReflectNamespace(namespace, informer)
	}

	c.waitGroup.Add(1)
	go func() {
		c.informerFactories[namespace].Start(c.stop)
		c.waitGroup.Done()
	}()
}

func (c *APIReflectorController) DispatchEvent(event reflectedApi.ApiEvent) error {
	return c.apiReflectors[event.Api].(reflectedApi.SpecializedAPIReflector).HandleEvent(event.Event)
}

func (c *APIReflectorController) Stop() {
	close(c.stop)
	c.waitGroup.Wait()
}
