package controller

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/incoming"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"sync"
)

type IncomingReflectorsController struct {
	*ReflectorsController
}

func NewIncomingReflectorsController(homeClient, foreignClient kubernetes.Interface,
	outputChan chan apimgmt.ApiEvent,
	namespaceNatting namespacesMapping.MapperController,
	opts map[options.OptionKey]options.Option) IncomingAPIReflectorsController {
	controller := &IncomingReflectorsController{
		&ReflectorsController{
			outputChan:               outputChan,
			homeClient:               homeClient,
			foreignClient:            foreignClient,
			homeInformerFactories:    make(map[string]informers.SharedInformerFactory),
			foreignInformerFactories: make(map[string]informers.SharedInformerFactory),
			apiReflectors:            make(map[apimgmt.ApiType]ri.APIReflector),
			namespaceNatting:         namespaceNatting,
			namespacedStops:          make(map[string]chan struct{}),
			homeWaitGroup:            &sync.WaitGroup{},
			foreignWaitGroup:         &sync.WaitGroup{},
		},
	}

	for api := range incoming.ApiMapping {
		controller.apiReflectors[api] = controller.buildIncomingReflector(api, opts)
	}

	return controller
}

func (c *IncomingReflectorsController) buildIncomingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector {
	apiReflector := &reflectors.GenericAPIReflector{
		Api:              api,
		OutputChan:       c.outputChan,
		ForeignClient:    c.foreignClient,
		LocalInformers:   make(map[string]cache.SharedIndexInformer),
		ForeignInformers: make(map[string]cache.SharedIndexInformer),
		NamespaceNatting: c.namespaceNatting,
	}
	specReflector := incoming.ApiMapping[api](apiReflector, opts)
	specReflector.SetSpecializedPreProcessingHandlers()

	return specReflector
}

func (c *IncomingReflectorsController) Start() {
	for {
		select {
		case ns := <-c.namespaceNatting.PollStartIncomingReflection():
			c.startNamespaceReflection(ns)
		case ns := <-c.namespaceNatting.PollStopIncomingReflection():
			c.stopNamespaceReflection(ns)
		default:
			break
		}
	}
}

func (c *IncomingReflectorsController) startNamespaceReflection(namespace string) {
	nattedNs, err := c.namespaceNatting.NatNamespace(namespace, false)
	if err != nil {
		klog.Errorf("error while natting namespace - ERR: %v", err)
		return
	}

	homeFactory := informers.NewSharedInformerFactoryWithOptions(c.homeClient, defaultResyncPeriod, informers.WithNamespace(namespace))
	foreignFactory := informers.NewSharedInformerFactoryWithOptions(c.foreignClient, defaultResyncPeriod, informers.WithNamespace(nattedNs))

	c.homeInformerFactories[namespace] = homeFactory
	c.foreignInformerFactories[nattedNs] = foreignFactory

	for api, handler := range incoming.InformerBuilders {
		homeInformer := handler(homeFactory)
		foreignInformer := handler(foreignFactory)

		homeIndexer := incoming.Indexers[api]
		if homeIndexer != nil {
			if err := homeInformer.AddIndexers(homeIndexer()); err != nil {
				klog.Errorf("Error while setting up home informer - ERR: %v", err)
			}
		}
		foreignIndexer := incoming.Indexers[api]
		if foreignIndexer != nil {
			if err := foreignInformer.AddIndexers(foreignIndexer()); err != nil {
				klog.Errorf("Error while setting up foreign informer - ERR: %v", err)
			}
		}

		c.apiReflectors[api].(ri.IncomingAPIReflector).SetInformers(ri.IncomingReflection, namespace, nattedNs, homeInformer, foreignInformer)
	}

	c.namespacedStops[namespace] = make(chan struct{})
	c.homeWaitGroup.Add(1)
	go func() {
		c.homeInformerFactories[namespace].Start(c.namespacedStops[namespace])

		<-c.namespacedStops[namespace]
		for _, reflector := range c.apiReflectors {
			reflector.(ri.IncomingAPIReflector).CleanupNamespace(namespace)
		}
		delete(c.homeInformerFactories, namespace)
		c.homeWaitGroup.Done()
	}()

	c.foreignWaitGroup.Add(1)
	go func() {
		c.foreignInformerFactories[nattedNs].Start(c.namespacedStops[namespace])

		<-c.namespacedStops[namespace]
		delete(c.foreignInformerFactories, nattedNs)
		c.foreignWaitGroup.Done()
	}()
}

func (c *IncomingReflectorsController) SetInforming(api apimgmt.ApiType, handler func(interface{})) {
	c.apiReflectors[api].(ri.APIReflector).SetInforming(handler)
}

func (c *IncomingReflectorsController) GetMirroredObject(api apimgmt.ApiType, namespace, name string) interface{} {
	return c.apiReflectors[api].(ri.IncomingAPIReflector).GetMirroredObject(namespace, name)
}

func (c *IncomingReflectorsController) ListMirroredObjects(api apimgmt.ApiType, namespace string) []interface{} {
	return c.apiReflectors[api].(ri.IncomingAPIReflector).ListMirroredObjects(namespace)
}

func (c *IncomingReflectorsController) stopNamespaceReflection(namespace string) {
	close(c.namespacedStops[namespace])
}
