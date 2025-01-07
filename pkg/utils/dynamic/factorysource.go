// Copyright 2019-2025 The Liqo Authors
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

package dynamic

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// NewFactorySource returns a new FactorySource.
func NewFactorySource(factory *RunnableFactory) *FactorySource {
	c := make(chan event.GenericEvent)
	handler := &factoryEventHandler{
		C: c,
	}
	return &FactorySource{
		handler: handler,
		c:       c,
		factory: factory,
	}
}

// FactorySource is a source that can be used to trigger a reconciliation.
type FactorySource struct {
	handler *factoryEventHandler
	c       chan event.GenericEvent
	factory *RunnableFactory
}

// Source returns a source that can be used to trigger a reconciliation.
func (f *FactorySource) Source(eh handler.EventHandler) source.Source {
	return source.Channel(f.c, eh)
}

// ForResource registers the handler for the given resource.
func (f *FactorySource) ForResource(gvr schema.GroupVersionResource) {
	_, err := f.factory.ForResource(gvr).Informer().AddEventHandler(f.handler)
	utilruntime.Must(err)
}

type factoryEventHandler struct {
	C chan event.GenericEvent
}

// OnAdd is called when an object is added.
func (h *factoryEventHandler) OnAdd(obj interface{}, _ bool) {
	unstructObj := obj.(*unstructured.Unstructured)
	h.C <- event.GenericEvent{
		Object: &networkingv1beta1.GatewayServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:            unstructObj.GetName(),
				Namespace:       unstructObj.GetNamespace(),
				OwnerReferences: unstructObj.GetOwnerReferences(),
				Labels:          unstructObj.GetLabels(),
				Annotations:     unstructObj.GetAnnotations(),
			},
		},
	}
}

// OnUpdate is called when an object is updated.
func (h *factoryEventHandler) OnUpdate(_, newObj interface{}) {
	unstructObj := newObj.(*unstructured.Unstructured)
	h.C <- event.GenericEvent{
		Object: &networkingv1beta1.GatewayServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:            unstructObj.GetName(),
				Namespace:       unstructObj.GetNamespace(),
				OwnerReferences: unstructObj.GetOwnerReferences(),
				Labels:          unstructObj.GetLabels(),
				Annotations:     unstructObj.GetAnnotations(),
			},
		},
	}
}

// OnDelete is called when an object is deleted.
func (h *factoryEventHandler) OnDelete(obj interface{}) {
	unstructObj := obj.(*unstructured.Unstructured)
	h.C <- event.GenericEvent{
		Object: &networkingv1beta1.GatewayServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:            unstructObj.GetName(),
				Namespace:       unstructObj.GetNamespace(),
				OwnerReferences: unstructObj.GetOwnerReferences(),
				Labels:          unstructObj.GetLabels(),
				Annotations:     unstructObj.GetAnnotations(),
			},
		},
	}
}
