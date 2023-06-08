// Copyright 2019-2023 The Liqo Authors
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

package netcfgcreator

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// ServiceWatcher reconciles Service objects to retrieve the Wireguard endpoint.
type ServiceWatcher struct {
	sync.RWMutex
	endpointIP   string
	endpointPort string

	configured bool
	wait       chan struct{}

	enqueuefn func(workqueue.RateLimitingInterface)
}

// NewServiceWatcher returns a new initialized ServiceWatcher instance.
func NewServiceWatcher(enqueuefn func(workqueue.RateLimitingInterface)) *ServiceWatcher {
	return &ServiceWatcher{
		configured: false,
		wait:       make(chan struct{}),

		enqueuefn: enqueuefn,
	}
}

// WiregardEndpoint returns the retrieved Wireguard endpoint information (IP/port).
func (sw *ServiceWatcher) WiregardEndpoint() (ip, port string) {
	sw.RLock()
	defer sw.RUnlock()

	return sw.endpointIP, sw.endpointPort
}

// WaitForConfigured waits until a valid key is retrieved for the first time.
func (sw *ServiceWatcher) WaitForConfigured(ctx context.Context) bool {
	sw.RLock()

	if !sw.configured {
		sw.RUnlock()
		klog.Info("Waiting for the configuration of the service watcher")

		select {
		case <-sw.wait:
			klog.Info("Service watcher correctly configured")
			return true
		case <-ctx.Done():
			klog.Warning("Context expired before configuring the service watcher")
			return false
		}
	}

	sw.RUnlock()
	return true
}

// Handlers returns the set of handlers used for the Watch configuration.
func (sw *ServiceWatcher) Handlers() handler.EventHandler {
	return handler.Funcs{
		CreateFunc: func(_ context.Context, ce event.CreateEvent, rli workqueue.RateLimitingInterface) {
			service := ce.Object.(*corev1.Service)
			sw.handle(service, rli)
		},
		UpdateFunc: func(_ context.Context, ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
			service := ue.ObjectNew.(*corev1.Service)
			sw.handle(service, rli)
		},
	}
}

// Predicates returns the set of predicates used for the Watch configuration.
func (sw *ServiceWatcher) Predicates() predicate.Predicate {
	secretsPredicate, err := predicate.LabelSelectorPredicate(liqolabels.GatewayServiceLabelSelector)
	utilruntime.Must(err)

	return secretsPredicate
}

// handle processes creation and update events of a Service object.
func (sw *ServiceWatcher) handle(service *corev1.Service, rli workqueue.RateLimitingInterface) {
	klog.V(4).Infof("Handling Service %q", klog.KObj(service))

	sw.Lock()
	defer sw.Unlock()

	var ip, port string
	var err error

	if ip, port, err = getters.RetrieveWGEPFromService(service, liqoconst.GatewayServiceAnnotationKey,
		liqoconst.DriverName); err != nil {
		klog.Error(err)
		return
	}

	// The endpoint did not change, nothing to do
	if ip == sw.endpointIP && port == sw.endpointPort {
		return
	}

	// Configure the new key, and set as configured if not yet done
	klog.Infof("Wiregard endpoint correctly retrieved: %s:%s", ip, port)
	sw.endpointIP = ip
	sw.endpointPort = port
	if !sw.configured {
		close(sw.wait)
		sw.configured = true
	}

	// Enqueue all foreign clusters for update (which in turn update the respective network configs)
	sw.enqueuefn(rli)
}
