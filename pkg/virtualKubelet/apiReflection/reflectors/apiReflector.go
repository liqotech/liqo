package reflectors

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"strings"
)

type GenericAPIReflector struct {
	Api                   apimgmt.ApiType
	PreProcessingHandlers ri.PreProcessingHandlers
	OutputChan            chan apimgmt.ApiEvent
	informingFunc         func(obj interface{})

	ForeignClient kubernetes.Interface
	HomeClient    kubernetes.Interface

	LocalInformers   map[string]cache.SharedIndexInformer
	ForeignInformers map[string]cache.SharedIndexInformer
	NamespaceNatting namespacesMapping.NamespaceNatter
}

func (r *GenericAPIReflector) GetForeignClient() kubernetes.Interface {
	return r.ForeignClient
}

func (r *GenericAPIReflector) GetHomeClient() kubernetes.Interface {
	return r.HomeClient
}

func (r *GenericAPIReflector) LocalInformer(namespace string) cache.SharedIndexInformer {
	return r.LocalInformers[namespace]
}

func (r *GenericAPIReflector) ForeignInformer(namespace string) cache.SharedIndexInformer {
	return r.ForeignInformers[namespace]
}

func (r *GenericAPIReflector) NattingTable() namespacesMapping.NamespaceNatter {
	return r.NamespaceNatting
}

func (r *GenericAPIReflector) PreProcessIsAllowed(obj interface{}) bool {
	if r.PreProcessingHandlers.IsAllowed == nil {
		return true
	}
	return r.PreProcessingHandlers.IsAllowed(obj)
}

func (r *GenericAPIReflector) PreProcessAdd(obj interface{}) interface{} {
	if r.PreProcessingHandlers.AddFunc == nil {
		return obj
	}
	return r.PreProcessingHandlers.AddFunc(obj)
}

func (r *GenericAPIReflector) PreProcessUpdate(newObj, oldObj interface{}) interface{} {
	if r.PreProcessingHandlers.UpdateFunc == nil {
		return newObj
	}
	return r.PreProcessingHandlers.UpdateFunc(newObj, oldObj)
}

func (r *GenericAPIReflector) PreProcessDelete(obj interface{}) interface{} {
	if r.PreProcessingHandlers.DeleteFunc == nil {
		return obj
	}
	return r.PreProcessingHandlers.DeleteFunc(obj)
}

func (r *GenericAPIReflector) SetInformers(reflectionType ri.ReflectionType, namespace, nattedNs string, localInformer, foreignInformer cache.SharedIndexInformer) {
	handlers := &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if ok := r.PreProcessIsAllowed(obj); !ok {
				return
			}
			o := r.PreProcessAdd(obj)
			if o == nil {
				return
			}
			r.Inform(apimgmt.ApiEvent{
				Event: watch.Event{
					Type:   watch.Added,
					Object: o.(runtime.Object),
				},
				Api: r.Api,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if ok := r.PreProcessIsAllowed(newObj); !ok {
				return
			}
			o := r.PreProcessUpdate(newObj, oldObj)
			if o == nil {
				return
			}
			r.Inform(apimgmt.ApiEvent{
				Event: watch.Event{
					Type:   watch.Modified,
					Object: o.(runtime.Object),
				},
				Api: r.Api,
			})
		},
		DeleteFunc: func(obj interface{}) {
			if ok := r.PreProcessIsAllowed(obj); !ok {
				return
			}
			o := r.PreProcessDelete(obj)
			if o == nil {
				return
			}
			r.Inform(apimgmt.ApiEvent{
				Event: watch.Event{
					Type:   watch.Deleted,
					Object: o.(runtime.Object),
				},
				Api: r.Api,
			})
		}}

	switch reflectionType {
	case ri.OutgoingReflection:
		localInformer.AddEventHandler(handlers)
	case ri.IncomingReflection:
		foreignInformer.AddEventHandler(handlers)
	}

	r.LocalInformers[namespace] = localInformer
	r.ForeignInformers[nattedNs] = foreignInformer
}

func (r *GenericAPIReflector) Inform(obj apimgmt.ApiEvent) {
	r.OutputChan <- obj
}

func (r *GenericAPIReflector) SetInforming(handler func(interface{})) {
	r.informingFunc = handler
}

func (r *GenericAPIReflector) PushToInforming(obj interface{}) {
	if r.informingFunc != nil {
		r.informingFunc(obj)
	} else {
		klog.V(3).Info("cannot push object to informing function, not existing yet")
	}
}

func (r *GenericAPIReflector) SetPreProcessingHandlers(handlers ri.PreProcessingHandlers) {
	r.PreProcessingHandlers = handlers
}

func (r *GenericAPIReflector) Keyer(namespace, name string) string {
	return strings.Join([]string{namespace, name}, "/")
}

func (r *GenericAPIReflector) GetObjFromForeignCache(namespace, key string) (interface{}, error) {
	informer := r.ForeignInformer(namespace)
	if informer == nil {
		return nil, errors.New("informer not yet instantiated")
	}

	obj, exists, err := informer.GetStore().GetByKey(key)
	if err != nil {
		return nil, errors.Wrap(err, "error while getting by key object from foreign cache")
	}
	if !exists {
		err = r.ForeignInformer(namespace).GetStore().Resync()
		if err != nil {
			return nil, errors.Wrap(err, "error while resyncing foreign cache")
		}
		obj, exists, err = r.ForeignInformer(namespace).GetStore().GetByKey(key)
		if err != nil {
			return nil, errors.Wrap(err, "error while retrieving object from foreign cache")
		}
		if !exists {
			return nil, errors.New("object not found after cache resync")
		}
	}

	return obj, nil
}
