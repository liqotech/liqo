package controller

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/incoming"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage"
)

type IncomingReflectorsController struct {
	*ReflectorsController
}

func NewIncomingReflectorsController(homeClient, foreignClient kubernetes.Interface, cacheManager *storage.Manager,
	outputChan chan apimgmt.ApiEvent,
	namespaceNatting namespacesmapping.MapperController,
	opts map[options.OptionKey]options.Option) IncomingAPIReflectorsController {
	controller := &IncomingReflectorsController{
		&ReflectorsController{
			reflectionType:   ri.IncomingReflection,
			outputChan:       outputChan,
			homeClient:       homeClient,
			foreignClient:    foreignClient,
			apiReflectors:    make(map[apimgmt.ApiType]ri.APIReflector),
			namespaceNatting: namespaceNatting,
			namespacedStops:  make(map[string]chan struct{}),
			reflectionGroup:  &sync.WaitGroup{},
			cacheManager:     cacheManager,
		},
	}

	for api := range incoming.ReflectorBuilder {
		controller.apiReflectors[api] = controller.buildIncomingReflector(api, opts)
	}

	return controller
}

func (c *IncomingReflectorsController) buildIncomingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector {
	apiReflector := &reflectors.GenericAPIReflector{
		Api:              api,
		OutputChan:       c.outputChan,
		ForeignClient:    c.foreignClient,
		HomeClient:       c.homeClient,
		CacheManager:     c.cacheManager,
		NamespaceNatting: c.namespaceNatting,
	}
	specReflector := incoming.ReflectorBuilder[api](apiReflector, opts)
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
		}
	}
}

// SetInforming configures the handlers triggered for a certain API type by incoming reflection events.
func (c *IncomingReflectorsController) SetInforming(api apimgmt.ApiType, handler func(*corev1.Pod)) {
	c.apiReflectors[api].(ri.APIReflector).SetInforming(handler)
}

func (c *IncomingReflectorsController) stopNamespaceReflection(namespace string) {
	if isChanOpen(c.namespacedStops[namespace]) {
		close(c.namespacedStops[namespace])
		delete(c.namespacedStops, namespace)
	}
}
