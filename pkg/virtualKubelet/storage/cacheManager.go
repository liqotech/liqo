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
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/internal/utils/errdefs"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/utils"
)

type readyCaches struct {
	sync.RWMutex
	caches map[string]struct{}
}

// Manager is a structure which wraps the informers used to interact with the home
// and the foreign cluster, together with the associated caches.
type Manager struct {
	homeInformers    *NamespacedAPICaches
	foreignInformers *NamespacedAPICaches

	homeReadyCaches    readyCaches
	foreignReadyCaches readyCaches
}

func readinessKeyer(namespace string, api apimgmt.ApiType) string {
	return fmt.Sprintf("%s/%s", namespace, apimgmt.ApiNames[api])
}

// checkNamespaceCaching checks if the caching of the requested informers has been started. It caches the result
// for allowing a subsequent fast checks.
func checkNamespaceCaching(backoff *wait.Backoff, rc *readyCaches, caches *NamespacedAPICaches, namespace string, api apimgmt.ApiType) error {
	checkFunc := func() error {
		rc.RLock()
		if _, ok := rc.caches[readinessKeyer(namespace, api)]; ok {
			rc.RUnlock()
		} else {
			rc.RUnlock()

			// home cache checks
			if caches == nil {
				return errors.New("informers set to nil")
			}
			caches.RLock()
			defer caches.RUnlock()

			apiCache := caches.Namespace(namespace)
			if apiCache == nil {
				return errdefs.Unavailablef("informers for api %v in namespace %v do not exist", apimgmt.ApiNames[api], namespace)
			}
			informer := apiCache.informer(api)
			if informer == nil {
				return errdefs.Unavailablef("informer for api %v in namespace %v does not exist", apimgmt.ApiNames[api], namespace)
			}
			if !informer.HasSynced() {
				return errdefs.Unavailablef("informer for api %v in namespace %v not synced yet", apimgmt.ApiNames[api], namespace)
			}

			// cache readiness result for home cache
			rc.Lock()
			rc.caches[readinessKeyer(namespace, api)] = struct{}{}
			rc.Unlock()
		}

		return nil
	}

	// if no backoff specified, we call the checkFunction only once
	if backoff == nil {
		return checkFunc()
	}

	// if the backoff has been set, we call a retry function on the check function by using the passed backoff
	if err := retry.OnError(*backoff,
		func(err error) bool {
			klog.V(6).Infof("operation retried because of ERR: %s", err.Error())
			return errdefs.IsUnavailable(err)
		},
		checkFunc,
	); err != nil {
		return err
	}

	return nil
}

// NewManager creates a new Manager instance for a given tuple of home and foreign clients.
func NewManager(homeClient, foreignClient kubernetes.Interface, resyncPeriod time.Duration) *Manager {
	homeInformers := &NamespacedAPICaches{
		apiInformers:      make(map[string]*APICaches),
		informerFactories: make(map[string]informers.SharedInformerFactory),
		client:            homeClient,
		resyncPeriod:      resyncPeriod,
	}

	foreignInformers := &NamespacedAPICaches{
		apiInformers:      make(map[string]*APICaches),
		informerFactories: make(map[string]informers.SharedInformerFactory),
		client:            foreignClient,
		resyncPeriod:      resyncPeriod,
	}

	manager := &Manager{
		homeInformers:    homeInformers,
		foreignInformers: foreignInformers,
		homeReadyCaches: readyCaches{
			RWMutex: sync.RWMutex{},
			caches:  make(map[string]struct{}),
		},
		foreignReadyCaches: readyCaches{
			RWMutex: sync.RWMutex{},
			caches:  make(map[string]struct{}),
		},
	}

	return manager
}

// AddHomeNamespace adds a given home namespace to the list of observed ones.
func (cm *Manager) AddHomeNamespace(namespace string) error {
	if cm.homeInformers == nil {
		return errors.New("home informers set to nil")
	}

	return cm.homeInformers.AddNamespace(namespace)
}

// AddForeignNamespace adds a given foreign namespace to the list of observed ones.
func (cm *Manager) AddForeignNamespace(namespace string) error {
	if cm.foreignInformers == nil {
		return errors.New("foreign informers set to nil")
	}

	return cm.foreignInformers.AddNamespace(namespace)
}

// StartHomeNamespace starts observing a given home namespace (after having been added).
func (cm *Manager) StartHomeNamespace(homeNamespace string, stop chan struct{}) error {
	if cm.homeInformers == nil {
		return errors.New("home informers set to nil")
	}

	return cm.homeInformers.startNamespace(homeNamespace, stop)
}

// StartForeignNamespace starts observing a given foreign namespace (after having been added).
func (cm *Manager) StartForeignNamespace(foreignNamespace string, stop chan struct{}) error {
	if cm.foreignInformers == nil {
		return errors.New("foreign informers set to nil")
	}

	return cm.foreignInformers.startNamespace(foreignNamespace, stop)
}

// RemoveNamespace removes a namespace from the list of observed ones.
func (cm *Manager) RemoveNamespace(namespace string) {
	cm.homeInformers.removeNamespace(namespace)
	cm.foreignInformers.removeNamespace(namespace)
}

// AddHomeEventHandlers configures the handlers executed on events triggered by modifications of a given API in a home namespace.
func (cm *Manager) AddHomeEventHandlers(api apimgmt.ApiType, namespace string, handlers *cache.ResourceEventHandlerFuncs) error {
	cm.homeInformers.Lock()
	defer cm.homeInformers.Unlock()

	if cm.homeInformers == nil {
		return errors.New("home informer set to nil")
	}

	apiCache := cm.homeInformers.Namespace(namespace)
	if apiCache == nil {
		return kerrors.NewServiceUnavailable(fmt.Sprintf("home cache for api %v in namespace %v not existing", apimgmt.ApiNames[api], namespace))
	}

	informer := apiCache.informer(api)
	if informer == nil {
		return kerrors.NewServiceUnavailable(
			fmt.Sprintf("cannot set handlers, home informer for api %v in namespace %v does not exist", apimgmt.ApiNames[api], namespace))
	}

	informer.AddEventHandler(handlers)

	return nil
}

// AddForeignEventHandlers configures the handlers executed on events triggered by modifications of a given API in a foreign namespace.
func (cm *Manager) AddForeignEventHandlers(api apimgmt.ApiType, namespace string, handlers *cache.ResourceEventHandlerFuncs) error {
	cm.foreignInformers.Lock()
	defer cm.foreignInformers.Unlock()

	apiCache := cm.foreignInformers.Namespace(namespace)
	if apiCache == nil {
		return errors.Errorf("foreign cache for api %v in namespace %v not existing", apimgmt.ApiNames[api], namespace)
	}

	informer := apiCache.informer(api)
	if informer == nil {
		return errors.Errorf("cannot set handlers, foreign informer for api %v in namespace %v does not exist", apimgmt.ApiNames[api], namespace)
	}

	informer.AddEventHandler(handlers)

	return nil
}

// GetHomeNamespacedObject retrieves a given cached object from the home cluster.
func (cm *Manager) GetHomeNamespacedObject(api apimgmt.ApiType, namespace, name string) (interface{}, error) {
	err := checkNamespaceCaching(&defaultBackoff, &cm.homeReadyCaches, cm.homeInformers, namespace, api)
	if err != nil {
		return nil, err
	}
	cm.homeInformers.RLock()
	defer cm.homeInformers.RUnlock()

	apiCache := cm.homeInformers.Namespace(namespace)
	return apiCache.getAPI(api, utils.Keyer(namespace, name))
}

// GetForeignNamespacedObject retrieves a given cached object from the foreign cluster.
func (cm *Manager) GetForeignNamespacedObject(api apimgmt.ApiType, namespace, name string) (interface{}, error) {
	err := checkNamespaceCaching(&defaultBackoff, &cm.foreignReadyCaches, cm.foreignInformers, namespace, api)
	if err != nil {
		return nil, err
	}

	cm.foreignInformers.RLock()
	defer cm.foreignInformers.RUnlock()

	apiCache := cm.foreignInformers.Namespace(namespace)
	return apiCache.getAPI(api, utils.Keyer(namespace, name))
}

// ListHomeNamespacedObject lists the cached objects in a namespace in the home cluster.
func (cm *Manager) ListHomeNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	err := checkNamespaceCaching(&defaultBackoff, &cm.homeReadyCaches, cm.homeInformers, namespace, api)
	if err != nil {
		return nil, err
	}
	cm.homeInformers.RLock()
	defer cm.homeInformers.RUnlock()

	apiCache := cm.homeInformers.Namespace(namespace)
	objects, err := apiCache.listAPI(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

// ListForeignNamespacedObject lists the APIs in a namespace in the foreign cluster.
func (cm *Manager) ListForeignNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	err := checkNamespaceCaching(&defaultBackoff, &cm.foreignReadyCaches, cm.foreignInformers, namespace, api)
	if err != nil {
		return nil, err
	}
	cm.foreignInformers.RLock()
	defer cm.foreignInformers.RUnlock()

	apiCache := cm.foreignInformers.Namespace(namespace)
	objects, err := apiCache.listAPI(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

// GetHomeAPIByIndex lists the cached objects matching a given index, in a namespace in the home cache.
func (cm *Manager) GetHomeAPIByIndex(api apimgmt.ApiType, namespace, index string) (interface{}, error) {
	err := checkNamespaceCaching(&defaultBackoff, &cm.homeReadyCaches, cm.homeInformers, namespace, api)
	if err != nil {
		return nil, err
	}
	cm.homeInformers.RLock()
	defer cm.homeInformers.RUnlock()

	apiCache := cm.homeInformers.Namespace(namespace)
	objects, err := apiCache.listAPIByIndex(api, index)
	if err != nil {
		return nil, err
	}

	if len(objects) == 0 {
		return nil, errdefs.NotFound(fmt.Sprintf("no objects indexed with key %s found", index))
	}
	if len(objects) > 1 {
		return nil, errors.New("multiple objects indexed with the same index")
	}

	return objects[0], nil
}

// GetForeignAPIByIndex lists the cached objects matching a given index, in a namespace in the foreign cache.
func (cm *Manager) GetForeignAPIByIndex(api apimgmt.ApiType, namespace, index string) (interface{}, error) {
	err := checkNamespaceCaching(&defaultBackoff, &cm.foreignReadyCaches, cm.foreignInformers, namespace, api)
	if err != nil {
		return nil, err
	}
	cm.foreignInformers.RLock()
	defer cm.foreignInformers.RUnlock()

	apiCache := cm.foreignInformers.Namespace(namespace)
	objects, err := apiCache.listAPIByIndex(api, index)
	if err != nil {
		return nil, err
	}

	if len(objects) == 0 {
		return nil, errdefs.NotFound(fmt.Sprintf("no objects indexed with key %s found", index))
	}
	if len(objects) > 1 {
		return nil, errors.New("multiple objects indexed with the same index")
	}

	return objects[0], nil
}
