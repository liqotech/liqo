package storage

import (
	"fmt"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/utils"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sync"
)

type Manager struct {
	homeInformers    *NamespacedAPICaches
	foreignInformers *NamespacedAPICaches
}

func NewManager(homeClient, foreignClient kubernetes.Interface) *Manager {
	homeInformers := &NamespacedAPICaches{
		apiInformers:      make(map[string]*APICaches),
		informerFactories: make(map[string]informers.SharedInformerFactory),
		client:            homeClient,
		mutex:             sync.RWMutex{},
	}

	foreignInformers := &NamespacedAPICaches{
		apiInformers:      make(map[string]*APICaches),
		informerFactories: make(map[string]informers.SharedInformerFactory),
		client:            foreignClient,
		mutex:             sync.RWMutex{},
	}

	manager := &Manager{
		homeInformers:    homeInformers,
		foreignInformers: foreignInformers,
	}

	return manager
}

func (cm *Manager) AddHomeNamespace(namespace string) error {
	if cm.homeInformers == nil {
		return errors.New("home informers set to nil")
	}

	return cm.homeInformers.AddNamespace(namespace)
}

func (cm *Manager) AddForeignNamespace(namespace string) error {
	if cm.foreignInformers == nil {
		return errors.New("foreign informers set to nil")
	}

	return cm.foreignInformers.AddNamespace(namespace)
}

func (cm *Manager) StartHomeNamespace(homeNamespace string, stop chan struct{}) error {
	if cm.homeInformers == nil {
		return errors.New("home informers set to nil")
	}

	return cm.homeInformers.startNamespace(homeNamespace, stop)
}

func (cm *Manager) StartForeignNamespace(foreignNamespace string, stop chan struct{}) error {
	if cm.foreignInformers == nil {
		return errors.New("foreign informers set to nil")
	}

	return cm.foreignInformers.startNamespace(foreignNamespace, stop)
}

func (cm *Manager) RemoveNamespace(namespace string) {
	cm.homeInformers.removeNamespace(namespace)
	cm.foreignInformers.removeNamespace(namespace)
}

func (cm *Manager) AddHomeEventHandlers(api apimgmt.ApiType, namespace string, handlers *cache.ResourceEventHandlerFuncs) error {
	cm.homeInformers.mutex.Lock()
	defer cm.homeInformers.mutex.Unlock()

	if cm.homeInformers == nil {
		return errors.New("home informer set to nil")
	}

	apiCache := cm.homeInformers.Namespace(namespace)
	if apiCache == nil {
		return kerrors.NewServiceUnavailable(fmt.Sprintf("home cache for api %v in namespace %v not existing", apimgmt.ApiNames[api], namespace))
	}

	informer := apiCache.informer(api)
	if informer == nil {
		return kerrors.NewServiceUnavailable(fmt.Sprintf("cannot set handlers, home informer for api %v in namespace %v does not exist", apimgmt.ApiNames[api], namespace))
	}

	informer.AddEventHandler(handlers)

	return nil
}

func (cm *Manager) AddForeignEventHandlers(api apimgmt.ApiType, namespace string, handlers *cache.ResourceEventHandlerFuncs) error {
	cm.foreignInformers.mutex.Lock()
	defer cm.foreignInformers.mutex.Unlock()

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

func (cm *Manager) GetHomeNamespacedObject(api apimgmt.ApiType, namespace, name string) (interface{}, error) {
	if cm.homeInformers == nil {
		return nil, kerrors.NewServiceUnavailable("home informers set to nil")
	}

	cm.homeInformers.mutex.RLock()
	defer cm.homeInformers.mutex.RUnlock()

	apiCache := cm.homeInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, kerrors.NewServiceUnavailable(fmt.Sprintf("home cache for api %v in namespace %v set to nil", apimgmt.ApiNames[api], namespace))
	}

	return apiCache.getApi(api, utils.Keyer(namespace, name))
}

func (cm *Manager) GetForeignNamespacedObject(api apimgmt.ApiType, namespace, name string) (interface{}, error) {
	if cm.foreignInformers == nil {
		return nil, errors.New("foreign informers set to nil")
	}

	cm.foreignInformers.mutex.RLock()
	defer cm.foreignInformers.mutex.RUnlock()

	apiCache := cm.foreignInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, errors.Errorf("foreign cache for api %v in namespace %v set to nil", apimgmt.ApiNames[api], namespace)
	}

	return apiCache.getApi(api, utils.Keyer(namespace, name))
}

func (cm *Manager) ListHomeNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	if cm.homeInformers == nil {
		return nil, kerrors.NewServiceUnavailable("home informers set to nil")
	}

	cm.homeInformers.mutex.RLock()
	defer cm.homeInformers.mutex.RUnlock()

	apiCache := cm.homeInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, kerrors.NewServiceUnavailable(fmt.Sprintf("home cache for api %v in namespace %v set to nil", apimgmt.ApiNames[api], namespace))
	}

	objects, err := apiCache.listApi(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (cm *Manager) ListForeignNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	if cm.foreignInformers == nil {
		return nil, errors.New("foreign informers set to nil")
	}

	cm.foreignInformers.mutex.RLock()
	defer cm.foreignInformers.mutex.RUnlock()

	apiCache := cm.foreignInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, errors.Errorf("foreign cache for api %v in namespace %v set to nil", apimgmt.ApiNames[api], namespace)
	}

	objects, err := apiCache.listApi(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (cm *Manager) ResyncListHomeNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	if cm.homeInformers == nil {
		return nil, errors.New("home informers set to nil")
	}

	cm.homeInformers.mutex.RLock()
	defer cm.homeInformers.mutex.RLock()

	apiCache := cm.homeInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, errors.Errorf("cache for api %v in namespace %v not existing", apimgmt.ApiNames[api], namespace)
	}

	objects, err := apiCache.resyncListObjects(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (cm *Manager) ResyncListForeignNamespacedObject(api apimgmt.ApiType, namespace string) ([]interface{}, error) {
	if cm.foreignInformers == nil {
		return nil, errors.New("foreign informers set to nil")
	}

	cm.foreignInformers.mutex.RLock()
	defer cm.foreignInformers.mutex.RUnlock()

	apiCache := cm.foreignInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, errors.Errorf("cache for api %v in namespace %v not existing", apimgmt.ApiNames[api], namespace)
	}

	objects, err := apiCache.resyncListObjects(api)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (cm *Manager) ListHomeApiByIndex(api apimgmt.ApiType, namespace, index string) ([]interface{}, error) {
	if cm.homeInformers == nil {
		return nil, errors.New("home informers set to nil")
	}

	cm.homeInformers.mutex.RLock()
	defer cm.homeInformers.mutex.RLock()

	apiCache := cm.homeInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, errors.Errorf("cache for api %v in namespace %v not existing", apimgmt.ApiNames[api], namespace)
	}

	objects, err := apiCache.listApiByIndex(api, index)
	if err != nil {
		return nil, err
	}

	return objects, nil
}

func (cm *Manager) ListForeignApiByIndex(api apimgmt.ApiType, namespace, index string) ([]interface{}, error) {
	if cm.foreignInformers == nil {
		return nil, errors.New("foreign informers set to nil")
	}

	cm.foreignInformers.mutex.RLock()
	defer cm.foreignInformers.mutex.RUnlock()

	apiCache := cm.foreignInformers.Namespace(namespace)
	if apiCache == nil {
		return nil, errors.Errorf("foreign cache for api %v in namespace %v set to nil", apimgmt.ApiNames[api], namespace)
	}

	objects, err := apiCache.listApiByIndex(api, index)
	if err != nil {
		return nil, err
	}

	return objects, nil
}
