package controller

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/incoming"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/outgoing"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	reflectionCache "github.com/liqotech/liqo/pkg/virtualKubelet/storage"
)

type APIReflectorsController interface {
	Stop()
	DispatchEvent(event apimgmt.ApiEvent)
}

type SpecializedAPIReflectorsController interface {
	Start()
}

type OutGoingAPIReflectorsController interface {
	APIReflectorsController
	SpecializedAPIReflectorsController

	buildOutgoingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector
}

type IncomingAPIReflectorsController interface {
	APIReflectorsController
	SpecializedAPIReflectorsController

	buildIncomingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.IncomingAPIReflector
	SetInforming(api apimgmt.ApiType, handler func(*corev1.Pod))
}

type ReflectorsController struct {
	reflectionType   ri.ReflectionType
	outputChan       chan apimgmt.ApiEvent
	homeClient       kubernetes.Interface
	foreignClient    kubernetes.Interface
	cacheManager     *reflectionCache.Manager
	apiReflectors    map[apimgmt.ApiType]ri.APIReflector
	reflectionGroup  *sync.WaitGroup
	namespaceNatting namespacesmapping.MapperController
	namespacedStops  map[string]chan struct{}
}

func (c *ReflectorsController) startNamespaceReflection(namespace string) {
	nattedNs, err := c.namespaceNatting.NatNamespace(namespace)
	if err != nil {
		klog.Errorf("error while natting namespace - ERR: %v", err)
		return
	}

	c.namespacedStops[namespace] = make(chan struct{})

	if c.reflectionType == ri.IncomingReflection {
		if err := c.cacheManager.AddForeignNamespace(nattedNs); err != nil {
			klog.Errorf("error while reflecting new namespace - ERR: %v", err)
			return
		}

		for api := range incoming.ReflectorBuilder {
			c.apiReflectors[api].SetupHandlers(api, c.reflectionType, namespace, nattedNs)
		}

		if err := c.cacheManager.StartForeignNamespace(nattedNs, c.namespacedStops[namespace]); err != nil {
			klog.Errorf("error while starting namespace caching - ERR: %v", err)
			return
		}
	}

	if c.reflectionType == ri.OutgoingReflection {
		if err := c.cacheManager.AddHomeNamespace(namespace); err != nil {
			klog.Errorf("error while reflecting new namespace - ERR: %v", err)
			return
		}

		for api := range outgoing.ReflectorBuilders {
			c.apiReflectors[api].SetupHandlers(api, c.reflectionType, namespace, nattedNs)
		}

		if err := c.cacheManager.StartHomeNamespace(namespace, c.namespacedStops[namespace]); err != nil {
			klog.Errorf("error while starting namespace caching - ERR: %v", err)
			return
		}
	}

	c.reflectionGroup.Add(1)
	go func() {
		<-c.namespacedStops[namespace]
		for _, reflector := range c.apiReflectors {
			reflector.(ri.SpecializedAPIReflector).CleanupNamespace(namespace)
		}
		c.reflectionGroup.Done()
	}()
}

func (c *ReflectorsController) DispatchEvent(event apimgmt.ApiEvent) {
	c.apiReflectors[event.Api].(ri.SpecializedAPIReflector).HandleEvent(event.Event)
}

func (c *ReflectorsController) Stop() {
	for _, stop := range c.namespacedStops {
		if isChanOpen(stop) {
			close(stop)
		}
	}
	c.reflectionGroup.Wait()
}

func isChanOpen(ch chan struct{}) bool {
	open := true
	select {
	case _, open = <-ch:
	default:
	}
	return open
}
