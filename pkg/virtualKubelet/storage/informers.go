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
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

var InformerBuilders = map[apimgmt.ApiType]func(informers.SharedInformerFactory) cache.SharedIndexInformer{
	apimgmt.Configmaps:     configmapsInformerBuilder,
	apimgmt.EndpointSlices: endpointSlicesInformerBuilder,
	apimgmt.Pods:           podsInformerBuilder,
	apimgmt.ReplicaSets:    replicaSetsInformerBuilder,
	apimgmt.Services:       servicesInformerBuilder,
	apimgmt.Secrets:        secretsInformerBuilder,
}

func configmapsInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Core().V1().ConfigMaps().Informer()
}

func endpointSlicesInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Discovery().V1beta1().EndpointSlices().Informer()
}

func podsInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Core().V1().Pods().Informer()
}

func replicaSetsInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Apps().V1().ReplicaSets().Informer()
}

func servicesInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Core().V1().Services().Informer()
}

func secretsInformerBuilder(factory informers.SharedInformerFactory) cache.SharedIndexInformer {
	return factory.Core().V1().Secrets().Informer()
}
