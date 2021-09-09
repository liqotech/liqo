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

package controller

import (
	"sync"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/outgoing"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/storage"
)

type OutgoingReflectorsController struct {
	*ReflectorsController
}

func NewOutgoingReflectorsController(homeClient, foreignClient kubernetes.Interface, cacheManager *storage.Manager,
	outputChan chan apimgmt.ApiEvent,
	namespaceNatting namespacesmapping.MapperController,
	opts map[options.OptionKey]options.Option) OutGoingAPIReflectorsController {
	controller := &OutgoingReflectorsController{
		&ReflectorsController{
			reflectionType:   ri.OutgoingReflection,
			outputChan:       outputChan,
			homeClient:       homeClient,
			foreignClient:    foreignClient,
			apiReflectors:    make(map[apimgmt.ApiType]ri.APIReflector),
			namespaceNatting: namespaceNatting,
			namespacedStops:  make(map[string]chan struct{}),
			reflectionGroup:  &sync.WaitGroup{},
			cacheManager:     cacheManager,
		},
	}

	for api := range outgoing.ReflectorBuilders {
		controller.apiReflectors[api] = controller.buildOutgoingReflector(api, opts)
	}

	return controller
}

func (c *OutgoingReflectorsController) buildOutgoingReflector(api apimgmt.ApiType, opts map[options.OptionKey]options.Option) ri.OutgoingAPIReflector {
	apiReflector := &reflectors.GenericAPIReflector{
		Api:              api,
		OutputChan:       c.outputChan,
		ForeignClient:    c.foreignClient,
		HomeClient:       c.homeClient,
		CacheManager:     c.cacheManager,
		NamespaceNatting: c.namespaceNatting,
	}
	specReflector := outgoing.ReflectorBuilders[api](apiReflector, opts)
	specReflector.SetSpecializedPreProcessingHandlers()

	return specReflector
}

func (c *OutgoingReflectorsController) Start() {
	for {
		select {
		case ns := <-c.namespaceNatting.PollStartOutgoingReflection():
			c.startNamespaceReflection(ns)
			klog.V(2).Infof("outgoing reflection for namespace %v started", ns)
		case ns := <-c.namespaceNatting.PollStopOutgoingReflection():
			c.stopNamespaceReflection(ns)
			klog.V(2).Infof("incoming reflection for namespace %v started", ns)
		}
	}
}

func (c *OutgoingReflectorsController) stopNamespaceReflection(namespace string) {
	if isChanOpen(c.namespacedStops[namespace]) {
		close(c.namespacedStops[namespace])
		delete(c.namespacedStops, namespace)
	}
}
