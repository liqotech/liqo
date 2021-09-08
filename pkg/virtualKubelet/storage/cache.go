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

package storage

import (
	"errors"
	"sync"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	clientgocache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/utils"
)

var (
	defaultBackoff = retry.DefaultBackoff
)

type NamespacedAPICaches struct {
	sync.RWMutex

	apiInformers      map[string]*APICaches
	informerFactories map[string]informers.SharedInformerFactory
	client            kubernetes.Interface
	resyncPeriod      time.Duration
}

func (ac *NamespacedAPICaches) Namespace(namespace string) *APICaches {
	return ac.apiInformers[namespace]
}

func (ac *NamespacedAPICaches) AddNamespace(namespace string) error {
	ac.Lock()
	defer ac.Unlock()

	if ac.apiInformers == nil {
		return errors.New("informers map set to nil")
	}
	if ac.informerFactories == nil {
		return errors.New("informer factories map set to nil")
	}
	if ac.client == nil {
		return errors.New("client set to nil")
	}

	ac.apiInformers[namespace] = &APICaches{
		caches: make(map[apimgmt.ApiType]clientgocache.SharedIndexInformer),
	}

	factory := informers.NewSharedInformerFactoryWithOptions(ac.client, ac.resyncPeriod, informers.WithNamespace(namespace))
	for api, builder := range InformerBuilders {
		informer := builder(factory)
		if indexers, ok := InformerIndexers[api]; ok {
			if err := informer.AddIndexers(indexers()); err != nil {
				return err
			}
		}
		ac.apiInformers[namespace].caches[api] = informer
	}
	ac.informerFactories[namespace] = factory

	return nil
}

func (ac *NamespacedAPICaches) startNamespace(namespace string, stop chan struct{}) error {
	if ac.informerFactories == nil {
		return errors.New("informer factories map set to nil")
	}

	ac.informerFactories[namespace].Start(stop)
	return nil
}

func (ac *NamespacedAPICaches) removeNamespace(namespace string) {
	delete(ac.apiInformers, namespace)
	delete(ac.informerFactories, namespace)
}

// APICaches represents a set of informers for a set of APIs.
type APICaches struct {
	caches map[apimgmt.ApiType]clientgocache.SharedIndexInformer
}

// informer retrieves the cache for a specific api. If the cache does not exist, it returns nil.
func (cache *APICaches) informer(api apimgmt.ApiType) clientgocache.SharedIndexInformer {
	return cache.caches[api]
}

// getAPI gets a specific given object for a specific given api.
func (cache *APICaches) getAPI(api apimgmt.ApiType, key string) (interface{}, error) {
	return utils.GetObject(cache.caches[api], key, defaultBackoff)
}

// listAPIByIndex lists all the api matching a specific index.
func (cache *APICaches) listAPIByIndex(api apimgmt.ApiType, key string) ([]interface{}, error) {
	return utils.ListIndexedObjects(cache.caches[api], apimgmt.ApiNames[api], key)
}

// listAPI lists the content of a given cached api.
func (cache *APICaches) listAPI(api apimgmt.ApiType) ([]interface{}, error) {
	return utils.ListObjects(cache.caches[api])
}

// resyncListObjects resync the cache of a given api, then lists the content of that cached api.
func (cache *APICaches) resyncListObjects(api apimgmt.ApiType) ([]interface{}, error) {
	return utils.ResyncListObjects(cache.caches[api])
}
