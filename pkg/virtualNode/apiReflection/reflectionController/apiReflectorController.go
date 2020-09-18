package reflectionController

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sync"
	"time"
)

type APIReflectorController struct {
	outputChan        chan ApiEvent
	homeClient        kubernetes.Interface
	foreignClient     kubernetes.Interface
	informerFactories map[string]informers.SharedInformerFactory
	apiReflectors     map[ApiType]APIReflector
	waitGroup             *sync.WaitGroup
	stop chan struct{}
}

func NewAPIReflectorController(homeClient, foreignClient kubernetes.Interface, outputChan chan ApiEvent) *APIReflectorController {
	controller := &APIReflectorController{
		outputChan:        outputChan,
		homeClient:        homeClient,
		foreignClient:     foreignClient,
		informerFactories: make(map[string]informers.SharedInformerFactory),
		apiReflectors:     make(map[ApiType]APIReflector),
		stop: make(chan struct{}),
	}

	for api := range apiMapping {
		controller.buildApiReflector(api)
	}

	return controller
}

func (c *APIReflectorController) buildApiReflector(api ApiType) APIReflector {
	apiReflector := &GenericAPIReflector{
		api:                   api,
		preProcessingHandlers: apiPreProcessingHandlers[api],
		outputChan:            c.outputChan,
		foreignClient:         c.foreignClient,
		informers:             make(map[string]cache.SharedIndexInformer),
	}
	return apiMapping[api](apiReflector)
}

func (c *APIReflectorController) ReflectNamespace(namespace string,
	reSyncPeriod time.Duration,
	opts informers.SharedInformerOption) {

	factory := informers.NewSharedInformerFactoryWithOptions(c.homeClient, reSyncPeriod, informers.WithNamespace(namespace), opts)
	c.informerFactories[namespace] = factory

	for api, handler := range informerBuilding {
		informer := handler(factory)
		c.apiReflectors[api].reflectNamespace(namespace, informer)
	}

	c.waitGroup.Add(1)
	go func() {
		c.informerFactories[namespace].Start(c.stop)
		c.waitGroup.Done()
	}()
}

func (c *APIReflectorController) DispatchEvent(event ApiEvent) error {
	return c.apiReflectors[event.api].(SpecializedAPIReflector).HandleEvent(event.event)
}

func (c *APIReflectorController) Stop() {
	close(c.stop)
	c.waitGroup.Wait()
}

