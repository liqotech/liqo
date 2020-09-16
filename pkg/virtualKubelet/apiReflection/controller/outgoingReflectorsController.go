package controller

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/outgoing"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"sync"
)

type OutgoingReflectorsController struct {
	*ReflectorsController
}

func NewOutgoingReflectorsController(homeClient, foreignClient kubernetes.Interface,
	outputChan chan apimgmt.ApiEvent,
	namespaceNatting namespacesMapping.MapperController,
	opts map[options.OptionKey]options.Option) OutGoingAPIReflectorsController {
	controller := &OutgoingReflectorsController{
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

	for api := range outgoing.ApiMapping {
		controller.apiReflectors[api] = controller.buildOutgoingReflector(api, opts)
	}

	return controller
}

func (c *OutgoingReflectorsController) buildOutgoingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	apiReflector := &reflectors.GenericAPIReflector{
		Api:              api,
		OutputChan:       c.outputChan,
		ForeignClient:    c.foreignClient,
		LocalInformers:   make(map[string]cache.SharedIndexInformer),
		ForeignInformers: make(map[string]cache.SharedIndexInformer),
		NamespaceNatting: c.namespaceNatting,
	}
	specReflector := outgoing.ApiMapping[api](apiReflector, opts)
	specReflector.SetSpecializedPreProcessingHandlers()

	return specReflector
}

func (c *OutgoingReflectorsController) Start() {
	for {
		select {
		case ns := <-c.namespaceNatting.PollStartOutgoingReflection():
			c.startNamespaceReflection(ns)
		case ns := <-c.namespaceNatting.PollStopOutgoingReflection():
			c.stopNamespaceReflection(ns)
		default:
			break
		}
	}
}

func (c *OutgoingReflectorsController) startNamespaceReflection(namespace string) {
	nattedNs, err := c.namespaceNatting.NatNamespace(namespace, false)
	if err != nil {
		klog.Errorf("error while natting namespace - ERR: %v", err)
		return
	}

	homeFactory := informers.NewSharedInformerFactoryWithOptions(c.homeClient, defaultResyncPeriod, informers.WithNamespace(namespace))
	foreignFactory := informers.NewSharedInformerFactoryWithOptions(c.foreignClient, defaultResyncPeriod, informers.WithNamespace(nattedNs))

	c.homeInformerFactories[namespace] = homeFactory
	c.foreignInformerFactories[nattedNs] = foreignFactory

	for api, handler := range outgoing.HomeInformerBuilders {
		homeInformer := handler(homeFactory)
		var foreignInformer cache.SharedIndexInformer

		foreignHandler, ok := outgoing.ForeignInformerBuilders[api]
		if ok {
			foreignInformer = foreignHandler(foreignFactory)
		} else {
			foreignInformer = handler(foreignFactory)
		}

		homeIndexer := outgoing.HomeIndexers[api]
		foreignIndexer, ok := outgoing.ForeignIndexers[api]
		if !ok {
			foreignIndexer = homeIndexer
		}

		if homeIndexer != nil {
			if err := homeInformer.AddIndexers(homeIndexer()); err != nil {
				klog.Errorf("Error while setting up home informer - ERR: %v", err)
			}
		}
		if foreignIndexer != nil {
			if err := foreignInformer.AddIndexers(foreignIndexer()); err != nil {
				klog.Errorf("Error while setting up foreign informer - ERR: %v", err)
			}
		}

		c.apiReflectors[api].(ri.OutgoingAPIReflector).SetInformers(ri.OutgoingReflection, namespace, nattedNs, homeInformer, foreignInformer)
	}

	c.namespacedStops[namespace] = make(chan struct{})
	c.homeWaitGroup.Add(1)
	go func() {
		c.homeInformerFactories[namespace].Start(c.namespacedStops[namespace])

		<-c.namespacedStops[namespace]
		for _, reflector := range c.apiReflectors {
			reflector.(ri.OutgoingAPIReflector).CleanupNamespace(namespace)
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

	klog.V(2).Infof("Outgoing reflection for namespace %v started", namespace)
}

func (c *OutgoingReflectorsController) stopNamespaceReflection(namespace string) {
	close(c.namespacedStops[namespace])
}
