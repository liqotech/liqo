package storage

import (
	"errors"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/utils"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"sync"
	"time"
)

var (
	defaultResyncPeriod = 10 * time.Second
	defaultBackoff      = retry.DefaultBackoff
)

type NamespacedAPICaches struct {
	apiInformers      map[string]*APICaches
	informerFactories map[string]informers.SharedInformerFactory
	client            kubernetes.Interface

	mutex sync.RWMutex
}

func (ac *NamespacedAPICaches) Namespace(namespace string) *APICaches {
	return ac.apiInformers[namespace]
}

func (ac *NamespacedAPICaches) AddNamespace(namespace string) error {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

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
		caches: make(map[apimgmt.ApiType]cache.SharedIndexInformer),
	}

	factory := informers.NewSharedInformerFactoryWithOptions(ac.client, defaultResyncPeriod, informers.WithNamespace(namespace))
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

// APICaches represents a set of informers for a set of APIs
type APICaches struct {
	caches map[apimgmt.ApiType]cache.SharedIndexInformer
}

// informer retrieves the cache for a specific api. If the cache does not exist, it returns nil
func (cache *APICaches) informer(api apimgmt.ApiType) cache.SharedIndexInformer {
	return cache.caches[api]
}

// getApi gets a specific given object for a specific given api
func (cache *APICaches) getApi(api apimgmt.ApiType, key string) (interface{}, error) {
	return utils.GetObject(cache.caches[api], key, defaultBackoff)
}

// listApiByIndex lists all the api matching a specific index
func (cache *APICaches) listApiByIndex(api apimgmt.ApiType, key string) ([]interface{}, error) {
	return utils.ListIndexedObjects(cache.caches[api], apimgmt.ApiNames[api], key)
}

// listApi lists the content of a given cached api
func (cache *APICaches) listApi(api apimgmt.ApiType) ([]interface{}, error) {
	return utils.ListObjects(cache.caches[api])
}

// resyncListObjects resync the cache of a given api, then lists the content of that cached api
func (cache *APICaches) resyncListObjects(api apimgmt.ApiType) ([]interface{}, error) {
	return utils.ResyncListObjects(cache.caches[api])
}
