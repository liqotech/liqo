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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/client-go/tools/cache"

	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

var InformerIndexers = map[apimgmt.ApiType]func() cache.Indexers{
	apimgmt.Configmaps:     configmapsIndexers,
	apimgmt.EndpointSlices: endpointSlicesIndexers,
	apimgmt.Pods:           podsIndexers,
	apimgmt.ReplicaSets:    replicasetsIndexers,
	apimgmt.Secrets:        secretsIndexers,
	apimgmt.Services:       servicesIndexers,
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

func endpointSlicesIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["endpointslices"] = func(obj interface{}) ([]string, error) {
		endpointSlice, ok := obj.(*discoveryv1beta1.EndpointSlice)
		if !ok {
			return []string{}, errors.New("cannot convert obj to endpointslice")
		}
		return []string{
			strings.Join([]string{endpointSlice.Namespace, endpointSlice.Name}, "/"),
		}, nil
	}
	return i
}

func podsIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["pods"] = func(obj interface{}) ([]string, error) {
		po, ok := obj.(*corev1.Pod)
		if !ok {
			return []string{}, errors.New("cannot convert obj to pod")
		}
		var label string
		if po.Labels != nil {
			label = po.Labels[virtualKubelet.ReflectedpodKey]
		}

		indices := []string{
			strings.Join([]string{po.Namespace, po.Name}, "/"),
			po.Name,
		}
		if label != "" {
			indices = append(indices, label)
		}
		return indices, nil
	}
	return i
}

func replicasetsIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["replicasets"] = func(obj interface{}) ([]string, error) {
		replicaset, ok := obj.(*appsv1.ReplicaSet)
		if !ok {
			return []string{}, errors.New("cannot convert obj to replicaset")
		}
		return []string{
			strings.Join([]string{replicaset.Namespace, replicaset.Name}, "/"),
			replicaset.Name,
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

func servicesIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["services"] = func(obj interface{}) ([]string, error) {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			return []string{}, errors.New("cannot convert obj to service")
		}
		return []string{
			strings.Join([]string{svc.Namespace, svc.Name}, "/"),
		}, nil
	}
	return i
}
