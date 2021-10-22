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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

var InformerIndexers = map[apimgmt.ApiType]func() cache.Indexers{
	apimgmt.Configmaps: configmapsIndexers,
	apimgmt.Secrets:    secretsIndexers,
}

func configmapsIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["configmaps"] = func(obj interface{}) ([]string, error) {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return []string{}, errors.New("cannot convert obj to configmap")
		}
		return []string{
			strings.Join([]string{cm.Namespace, cm.Name}, "/"),
		}, nil
	}
	return i
}

func secretsIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["secrets"] = func(obj interface{}) ([]string, error) {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			return []string{}, errors.New("cannot convert obj to secret")
		}
		return []string{
			strings.Join([]string{secret.Namespace, secret.Name}, "/"),
		}, nil
	}
	return i
}
