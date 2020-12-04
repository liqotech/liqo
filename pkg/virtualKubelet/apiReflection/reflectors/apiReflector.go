package reflectors

import (
	"context"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	vkContext "github.com/liqotech/liqo/pkg/virtualKubelet/context"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"
	reflectionCache "github.com/liqotech/liqo/pkg/virtualKubelet/storage"
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

	CacheManager reflectionCache.CacheManagerReaderAdder

	NamespaceNatting namespacesMapping.NamespaceNatter
}

func (r *GenericAPIReflector) GetForeignClient() kubernetes.Interface {
	return r.ForeignClient
}

func (r *GenericAPIReflector) GetHomeClient() kubernetes.Interface {
	return r.HomeClient
}

func (r *GenericAPIReflector) GetCacheManager() reflectionCache.CacheManagerReader {
	return r.CacheManager
}

func (r *GenericAPIReflector) NattingTable() namespacesMapping.NamespaceNatter {
	return r.NamespaceNatting
}

func (r *GenericAPIReflector) PreProcessIsAllowed(ctx context.Context, obj interface{}) bool {
	if r.PreProcessingHandlers.IsAllowed == nil {
		return true
	}
	return r.PreProcessingHandlers.IsAllowed(ctx, obj)
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

func (r *GenericAPIReflector) SetupHandlers(api apimgmt.ApiType, reflectionType ri.ReflectionType, namespace, nattedNs string) {
	handlers := &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if ok := r.PreProcessIsAllowed(vkContext.SetIncomingMethod(context.TODO(), vkContext.IncomingAdded), obj); !ok {
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
			if ok := r.PreProcessIsAllowed(vkContext.SetIncomingMethod(context.TODO(), vkContext.IncomingModified), newObj); !ok {
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
			if ok := r.PreProcessIsAllowed(vkContext.SetIncomingMethod(context.TODO(), vkContext.IncomingDeleted), obj); !ok {
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
		if err := r.CacheManager.AddHomeEventHandlers(api, namespace, handlers); err != nil {
			klog.Errorf("error while setting up home Event handlers for api %v in namespace %v - ERR: %v", api, namespace, err)
		}
	case ri.IncomingReflection:
		if err := r.CacheManager.AddForeignEventHandlers(api, nattedNs, handlers); err != nil {
			klog.Errorf("error while setting up foreign Event handlers for api %v in namespace %v - ERR: %v", api, namespace, err)
		}
	}
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
