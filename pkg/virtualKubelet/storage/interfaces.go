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
	GetHomeAPIByIndex(apimgmt.ApiType, string, string) (interface{}, error)
	GetForeignAPIByIndex(apimgmt.ApiType, string, string) (interface{}, error)
}

type CacheManagerReaderAdder interface {
	CacheManagerAdder
	CacheManagerReader
}
