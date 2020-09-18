package reflectionController

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type PreProcessingHandlers struct {
	addFunc func(obj interface{}) interface{}
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

	reflectNamespace(namespace string, informer cache.SharedIndexInformer)
}

type SpecializedAPIReflector interface {
	HandleEvent(interface{}) error
}

type GenericAPIReflector struct {
	api                   ApiType
	preProcessingHandlers PreProcessingHandlers
	outputChan            chan ApiEvent

	foreignClient kubernetes.Interface
	informers     map[string]cache.SharedIndexInformer
}

func (r *GenericAPIReflector) PreProcessAdd(obj interface{}) interface{} {
	if r.preProcessingHandlers.updateFunc == nil {
		return obj
	}
	return r.preProcessingHandlers.addFunc(obj)
}

func (r *GenericAPIReflector) PreProcessUpdate(newObj, oldObj interface{}) interface{} {
	if r.preProcessingHandlers.updateFunc == nil {
		return newObj
	}
	return r.preProcessingHandlers.updateFunc(newObj, oldObj)
}

func (r *GenericAPIReflector) PreProcessDelete(obj interface{}) interface{} {
	if r.preProcessingHandlers.deleteFunc == nil {
		return obj
	}
	return r.preProcessingHandlers.deleteFunc(obj)
}

func (r *GenericAPIReflector) reflectNamespace(namespace string, informer cache.SharedIndexInformer) {
	informer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if r.PreProcessAdd(obj) == nil {
				return
			}
			r.Inform(ApiEvent{
				event: watch.Event{
					Type:   watch.Added,
					Object: obj.(runtime.Object),
				},
				api: r.api,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if r.PreProcessUpdate(oldObj, newObj) == nil {
				return
			}
			r.Inform(ApiEvent{
					event: watch.Event{
						Type:   watch.Modified,
						Object: newObj.(runtime.Object),
					},
					api: r.api,
			})
		},
		DeleteFunc: func(obj interface{}) {
			if r.PreProcessDelete(obj) == nil {
				return
			}
			r.Inform(ApiEvent{
				event: watch.Event{
					Object: obj.(runtime.Object),
				},
				api: r.api,
			})
		},
	})
	r.informers[namespace] = informer
}

func (r *GenericAPIReflector) Inform(obj ApiEvent) {
	r.outputChan <- obj
}
