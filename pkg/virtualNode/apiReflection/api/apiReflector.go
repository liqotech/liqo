package api

import (
	"github.com/liqotech/liqo/pkg/virtualNode/namespacesMapping"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PreProcessingHandlers struct {
	addFunc    func(obj interface{}) interface{}
	updateFunc func(newObj, oldObj interface{}) interface{}
	deleteFunc func(obj interface{}) interface{}
}

type APIPreProcessing interface {
	PreProcessAdd(obj interface{}) interface{}
	PreProcessUpdate(newObj, oldObj interface{}) interface{}
	PreProcessDelete(obj interface{}) interface{}
}

type APIReflector interface {
	APIPreProcessing

	Inform(obj ApiEvent)

	ReflectNamespace(namespace string, informer cache.SharedIndexInformer)
}

type SpecializedAPIReflector interface {
	SetPreProcessingHandlers()
	HandleEvent(interface{}) error
}

type GenericAPIReflector struct {
	Api                   ApiType
	PreProcessingHandlers PreProcessingHandlers
	OutputChan            chan ApiEvent

	ForeignClient    kubernetes.Interface
	Informers        map[string]cache.SharedIndexInformer
	NamespaceNatting namespacesMapping.NamespaceNatter
}

func (r *GenericAPIReflector) PreProcessAdd(obj interface{}) interface{} {
	if r.PreProcessingHandlers.updateFunc == nil {
		return obj
	}
	return r.PreProcessingHandlers.addFunc(obj)
}

func (r *GenericAPIReflector) PreProcessUpdate(newObj, oldObj interface{}) interface{} {
	if r.PreProcessingHandlers.updateFunc == nil {
		return newObj
	}
	return r.PreProcessingHandlers.updateFunc(newObj, oldObj)
}

func (r *GenericAPIReflector) PreProcessDelete(obj interface{}) interface{} {
	if r.PreProcessingHandlers.deleteFunc == nil {
		return obj
	}
	return r.PreProcessingHandlers.deleteFunc(obj)
}

func (r *GenericAPIReflector) ReflectNamespace(namespace string, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if r.PreProcessAdd(obj) == nil {
				return
			}
			r.Inform(ApiEvent{
				Event: watch.Event{
					Type:   watch.Added,
					Object: obj.(runtime.Object),
				},
				Api: r.Api,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if r.PreProcessUpdate(oldObj, newObj) == nil {
				return
			}
			r.Inform(ApiEvent{
				Event: watch.Event{
					Type:   watch.Modified,
					Object: newObj.(runtime.Object),
				},
				Api: r.Api,
			})
		},
		DeleteFunc: func(obj interface{}) {
			if r.PreProcessDelete(obj) == nil {
				return
			}
			r.Inform(ApiEvent{
				Event: watch.Event{
					Object: obj.(runtime.Object),
				},
				Api: r.Api,
			})
		},
	})
	r.Informers[namespace] = informer
}

func (r *GenericAPIReflector) Inform(obj ApiEvent) {
	r.OutputChan <- obj
}
