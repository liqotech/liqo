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

package crdclient

import (
	"fmt"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// WatchResources is a wrapper cache function that allows to create either a real cache
// or a fake one, depending on the global variable Fake.
func WatchResources(clientSet NamespacedCRDClientInterface,
	resource, namespace string,
	resyncPeriod time.Duration,
	handlers cache.ResourceEventHandlerFuncs,
	lo metav1.ListOptions) (cache.Store, chan struct{}, error) {
	if Fake {
		return WatchfakeResources(resource, handlers)
	} else {
		return WatchRealResources(clientSet, resource, namespace, resyncPeriod, handlers, lo)
	}
}

// Watch RealResources creates.
func WatchRealResources(clientSet NamespacedCRDClientInterface,
	resource, namespace string,
	resyncPeriod time.Duration,
	handlers cache.ResourceEventHandlerFuncs,
	lo metav1.ListOptions) (cache.Store, chan struct{}, error) {
	listFunc := func(ls metav1.ListOptions) (result runtime.Object, err error) {
		ls = lo
		return clientSet.Resource(resource).Namespace(namespace).List(&ls)
	}

	watchFunc := func(ls metav1.ListOptions) (watch.Interface, error) {
		ls = lo
		return clientSet.Resource(resource).Namespace(namespace).Watch(&ls)
	}
	res, ok := Registry[resource]
	if !ok {
		return nil, nil, fmt.Errorf("reflection for api %v not set", resource)
	}
	t := reflect.New(res.SingularType).Interface().(runtime.Object)

	store, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  listFunc,
			WatchFunc: watchFunc,
		},
		t,
		resyncPeriod,
		handlers,
	)

	stopChan := make(chan struct{}, 1)

	go controller.Run(stopChan)

	return store, stopChan, nil
}

// WatchfakeResources creates a Fake custom informer, useful for testing purposes
// TODO: to implement all the caching functionality, such as resync, filtering, etc.
func WatchfakeResources(resource string, handlers cache.ResourceEventHandlerFuncs) (cache.Store, chan struct{}, error) {
	res, ok := Registry[resource]
	if !ok {
		return nil, nil, fmt.Errorf("reflection for api %v not set", resource)
	}

	store, stop := NewFakeCustomInformer(handlers, res.Keyer, res.Resource)
	return store, stop, nil
}
