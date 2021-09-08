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
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type KeyerFunc func(obj runtime.Object) (string, error)

type RegistryType struct {
	SingularType reflect.Type
	PluralType   reflect.Type

	Keyer    KeyerFunc
	Resource schema.GroupResource
}

var Registry = make(map[string]RegistryType)

func AddToRegistry(api string, singular, plural runtime.Object, keyer KeyerFunc, resource schema.GroupResource) {
	Registry[api] = RegistryType{
		SingularType: reflect.TypeOf(singular).Elem(),
		PluralType:   reflect.TypeOf(plural).Elem(),
		Keyer:        keyer,
		Resource:     resource,
	}
}
