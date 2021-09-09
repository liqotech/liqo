// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reflectors

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	vkContext "github.com/liqotech/liqo/pkg/virtualKubelet/context"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	reflectionCache "github.com/liqotech/liqo/pkg/virtualKubelet/storage"
)

type GenericAPIReflector struct {
	Api                   apimgmt.ApiType
	PreProcessingHandlers ri.PreProcessingHandlers
	OutputChan            chan apimgmt.ApiEvent
	informingFunc         func(pod *corev1.Pod)

	ForeignClient kubernetes.Interface
	HomeClient    kubernetes.Interface

	CacheManager reflectionCache.CacheManagerReaderAdder

	NamespaceNatting namespacesmapping.NamespaceNatter
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

// NattingTable returns a namespaceNatter object to handle namespace translations.
func (r *GenericAPIReflector) NattingTable() namespacesmapping.NamespaceNatter {
	return r.NamespaceNatting
}

func (r *GenericAPIReflector) PreProcessIsAllowed(ctx context.Context, obj interface{}) bool {
	if r.PreProcessingHandlers.IsAllowed == nil {
		return true
	}
	return r.PreProcessingHandlers.IsAllowed(ctx, obj)
}

func (r *GenericAPIReflector) PreProcessAdd(obj interface{}) (interface{}, watch.EventType) {
	if r.PreProcessingHandlers.AddFunc == nil {
		return obj, watch.Added
	}
	return r.PreProcessingHandlers.AddFunc(obj)
}

func (r *GenericAPIReflector) PreProcessUpdate(newObj, oldObj interface{}) (interface{}, watch.EventType) {
	if r.PreProcessingHandlers.UpdateFunc == nil {
		return newObj, watch.Modified
	}
	return r.PreProcessingHandlers.UpdateFunc(newObj, oldObj)
}

func (r *GenericAPIReflector) PreProcessDelete(obj interface{}) (interface{}, watch.EventType) {
	if r.PreProcessingHandlers.DeleteFunc == nil {
		return obj, watch.Deleted
	}
	return r.PreProcessingHandlers.DeleteFunc(obj)
}

func (r *GenericAPIReflector) SetupHandlers(api apimgmt.ApiType, reflectionType ri.ReflectionType, namespace, nattedNs string) {
	handlers := &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if ok := r.PreProcessIsAllowed(vkContext.SetIncomingMethod(context.TODO(), vkContext.IncomingAdded), obj); !ok {
				return
			}
			o, event := r.PreProcessAdd(obj)
			if o == nil {
				return
			}
			r.Inform(apimgmt.ApiEvent{
				Event: watch.Event{
					Type:   event,
					Object: o.(runtime.Object),
				},
				Api: r.Api,
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if ok := r.PreProcessIsAllowed(vkContext.SetIncomingMethod(context.TODO(), vkContext.IncomingModified), newObj); !ok {
				return
			}
			o, event := r.PreProcessUpdate(newObj, oldObj)
			if o == nil {
				return
			}
			r.Inform(apimgmt.ApiEvent{
				Event: watch.Event{
					Type:   event,
					Object: o.(runtime.Object),
				},
				Api: r.Api,
			})
		},
		DeleteFunc: func(obj interface{}) {
			if ok := r.PreProcessIsAllowed(vkContext.SetIncomingMethod(context.TODO(), vkContext.IncomingDeleted), obj); !ok {
				return
			}
			o, event := r.PreProcessDelete(obj)
			if o == nil {
				return
			}
			r.Inform(apimgmt.ApiEvent{
				Event: watch.Event{
					Type:   event,
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

// SetInforming configures the handlers triggered for a certain API type by incoming reflection events.
func (r *GenericAPIReflector) SetInforming(handler func(*corev1.Pod)) {
	r.informingFunc = handler
}

// PushToInforming pushes a pod to the informing function.
func (r *GenericAPIReflector) PushToInforming(pod *corev1.Pod) {
	if r.informingFunc != nil {
		r.informingFunc(pod)
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
