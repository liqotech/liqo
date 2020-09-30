package controller

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"sync"
	"time"
)

type APIReflectorsController interface {
	Start()
	Stop()
	DispatchEvent(event apimgmt.ApiEvent)
}

type OutGoingAPIReflectorsController interface {
	APIReflectorsController

	buildOutgoingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector
}

type IncomingAPIReflectorsController interface {
	APIReflectorsController

	buildIncomingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector

	SetInforming(api apimgmt.ApiType, handler func(interface{}))
	GetMirroredObject(api apimgmt.ApiType, namespace, name string) interface{}
	ListMirroredObjects(api apimgmt.ApiType, namespace string) []interface{}
}

var defaultResyncPeriod = 10 * time.Second

type ReflectorsController struct {
	outputChan               chan apimgmt.ApiEvent
	homeClient               kubernetes.Interface
	foreignClient            kubernetes.Interface
	homeInformerFactories    map[string]informers.SharedInformerFactory
	foreignInformerFactories map[string]informers.SharedInformerFactory

	apiReflectors map[apimgmt.ApiType]ri.APIReflector

	reflectionGroup  *sync.WaitGroup
	namespaceNatting namespacesMapping.MapperController
	namespacedStops  map[string]chan struct{}
}

func (c *ReflectorsController) DispatchEvent(event apimgmt.ApiEvent) {
	c.apiReflectors[event.Api].(ri.SpecializedAPIReflector).HandleEvent(event.Event)
}

func (c *ReflectorsController) Stop() {
	for _, stop := range c.namespacedStops {
		close(stop)
	}
	c.reflectionGroup.Wait()
}
