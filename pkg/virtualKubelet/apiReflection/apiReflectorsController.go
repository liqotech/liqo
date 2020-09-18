package apiReflection

import (
	reflectedApi "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/api"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sync"
	"time"
)

var defaultResyncPeriod = 10 * time.Second

type APIReflectorsController struct {
	outputChan        chan reflectedApi.ApiEvent
	homeClient        kubernetes.Interface
	foreignClient     kubernetes.Interface
	informerFactories map[string]informers.SharedInformerFactory
	apiReflectors     map[reflectedApi.ApiType]reflectedApi.APIReflector
	waitGroup         *sync.WaitGroup
	namespaceNatting  namespacesMapping.NamespaceController
	namespacedStops   map[string]chan struct{}
}

func NewAPIReflectorsController(homeClient, foreignClient kubernetes.Interface,
	outputChan chan reflectedApi.ApiEvent,
	namespaceNatting namespacesMapping.NamespaceController) *APIReflectorsController {
	controller := &APIReflectorsController{
		outputChan:        outputChan,
		homeClient:        homeClient,
		foreignClient:     foreignClient,
		informerFactories: make(map[string]informers.SharedInformerFactory),
		apiReflectors:     make(map[reflectedApi.ApiType]reflectedApi.APIReflector),
		namespaceNatting:  namespaceNatting,
		namespacedStops:   make(map[string]chan struct{}),
	}

	for api := range reflectedApi.ApiMapping {
		controller.buildApiReflector(api)
	}

	return controller
}

func (c *APIReflectorsController) Start() {
	for {
		select {
		case ns := <-c.namespaceNatting.PollStartReflection():
			c.startNamespaceReflection(ns)
		case ns := <-c.namespaceNatting.PollStopReflection():
			c.stopNamespaceReflection(ns)
		default:
		}
	}
}

func (c *APIReflectorsController) buildApiReflector(api reflectedApi.ApiType) reflectedApi.APIReflector {
	apiReflector := &reflectedApi.GenericAPIReflector{
		Api:              api,
		OutputChan:       c.outputChan,
		ForeignClient:    c.foreignClient,
		Informers:        make(map[string]cache.SharedIndexInformer),
		NamespaceNatting: c.namespaceNatting,
	}
	specReflector := reflectedApi.ApiMapping[api](apiReflector)
	specReflector.SetPreProcessingHandlers()

	return specReflector
}

func (c *APIReflectorsController) startNamespaceReflection(namespace string) {

	factory := informers.NewSharedInformerFactoryWithOptions(c.homeClient, defaultResyncPeriod, informers.WithNamespace(namespace))
	c.informerFactories[namespace] = factory

	for api, handler := range reflectedApi.InformerBuilding {
		informer := handler(factory)
		c.apiReflectors[api].ReflectNamespace(namespace, informer)
	}

	c.namespacedStops[namespace] = make(chan struct{})
	c.waitGroup.Add(1)
	go func() {
		c.informerFactories[namespace].Start(c.namespacedStops[namespace])
		c.waitGroup.Done()
	}()
}

func (c *APIReflectorsController) stopNamespaceReflection(namespace string) {
	close(c.namespacedStops[namespace])
}

func (c *APIReflectorsController) DispatchEvent(event reflectedApi.ApiEvent) {
	c.apiReflectors[event.Api].(reflectedApi.SpecializedAPIReflector).HandleEvent(event.Event)
}

func (c *APIReflectorsController) Stop() {
	for _, stop := range c.namespacedStops {
		close(stop)
	}
	c.waitGroup.Wait()
}
