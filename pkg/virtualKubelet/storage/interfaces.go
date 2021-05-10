package storage

import (
	"k8s.io/client-go/tools/cache"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

type APICacheInterface interface {
	informer(apimgmt.ApiType) cache.SharedIndexInformer
	getApi(apimgmt.ApiType, string) (interface{}, error)
	listApiByIndex(apimgmt.ApiType, string) ([]interface{}, error)
	listApi(apimgmt.ApiType) ([]interface{}, error)
	resyncListObjects(apimgmt.ApiType) ([]interface{}, error)
}

type CacheManagerAdder interface {
	AddHomeNamespace(string) error
	AddForeignNamespace(string) error
	StartHomeNamespace(string, chan struct{}) error
	StartForeignNamespace(string, chan struct{}) error
	RemoveNamespace(string)
	AddHomeEventHandlers(apimgmt.ApiType, string, *cache.ResourceEventHandlerFuncs) error
	AddForeignEventHandlers(apimgmt.ApiType, string, *cache.ResourceEventHandlerFuncs) error
}

type CacheManagerReader interface {
	GetHomeNamespacedObject(apimgmt.ApiType, string, string) (interface{}, error)
	GetForeignNamespacedObject(apimgmt.ApiType, string, string) (interface{}, error)
	ListHomeNamespacedObject(apimgmt.ApiType, string) ([]interface{}, error)
	ListForeignNamespacedObject(apimgmt.ApiType, string) ([]interface{}, error)
	GetHomeApiByIndex(apimgmt.ApiType, string, string) (interface{}, error)
	GetForeignApiByIndex(apimgmt.ApiType, string, string) (interface{}, error)
}

type CacheManagerReaderAdder interface {
	CacheManagerAdder
	CacheManagerReader
}
