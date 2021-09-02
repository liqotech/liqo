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

package netcfgcreator

import (
	"context"
	"strconv"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
)

// These labels are the ones set during the deployment of liqo using the helm chart.
// Any change to those labels on the helm chart has also to be reflected here.
var (
	serviceLabelKey      = "net.liqo.io/gateway"
	serviceLabelValue    = "true"
	serviceAnnotationKey = "net.liqo.io/gatewayNodeIP"
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
		CreateFunc: func(ce event.CreateEvent, rli workqueue.RateLimitingInterface) {
			service := ce.Object.(*corev1.Service)
			sw.handle(service, rli)
		},
		UpdateFunc: func(ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
			service := ue.ObjectNew.(*corev1.Service)
			sw.handle(service, rli)
		},
	}
}

// Predicates returns the set of predicates used for the Watch configuration.
func (sw *ServiceWatcher) Predicates() predicate.Predicate {
	secretsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      serviceLabelKey,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{serviceLabelValue},
		}},
	})
	utilruntime.Must(err)

	return secretsPredicate
}

// handle processes creation and update events of a Service object.
func (sw *ServiceWatcher) handle(service *corev1.Service, rli workqueue.RateLimitingInterface) {
	klog.V(4).Infof("Handling Service %q", klog.KObj(service))

	sw.Lock()
	defer sw.Unlock()

	var ip, port string
	var retrieved bool

	switch service.Spec.Type {
	case corev1.ServiceTypeNodePort:
		ip, port, retrieved = sw.retrieveFromNodePort(service)
	case corev1.ServiceTypeLoadBalancer:
		ip, port, retrieved = sw.retrieveFromLoadBalancer(service)
	default:
		klog.Errorf("Service %q is of type %s, only types of %s and %s are accepted",
			klog.KObj(service), service.Spec.Type, corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort)
		return
	}

	// The endpoint did not change, nothing to do
	if !retrieved || (ip == sw.endpointIP && port == sw.endpointPort) {
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

// retrieveFromNodePort retrieves the Wireguard endpoint from a NodePort service.
func (sw *ServiceWatcher) retrieveFromNodePort(service *corev1.Service) (endpointIP, endpointPort string, retrieved bool) {
	// Check if the node's IP where the gatewayPod is running has been set
	endpointIP, retrieved = service.GetAnnotations()[serviceAnnotationKey]
	if !retrieved {
		klog.Warningf("The node IP where the gateway pod is running has not yet been set as an annotation for service %q", klog.KObj(service))
		return endpointIP, endpointPort, false
	}

	// Check if the nodePort for wireguard has been set
	for _, port := range service.Spec.Ports {
		if port.Name == wireguard.DriverName {
			if port.NodePort == 0 {
				klog.Warningf("The NodePort for service %s has not yet been set", klog.KObj(service))
				return endpointIP, endpointPort, false
			}
			endpointPort = strconv.FormatInt(int64(port.NodePort), 10)
			return endpointIP, endpointPort, true
		}
	}

	klog.Warningf("Port %s not found in service %q", wireguard.DriverName, klog.KObj(service))
	return endpointIP, endpointPort, false
}

// retrieveFromLoadBalancer retrieves the Wireguard endpoint from a LoadBalancer service.
func (sw *ServiceWatcher) retrieveFromLoadBalancer(service *corev1.Service) (endpointIP, endpointPort string, retrieved bool) {
	// Check if the ingress IP has been set
	if len(service.Status.LoadBalancer.Ingress) == 0 {
		klog.Warningf("The ingress IP has not been set for service %q of type %s", klog.KObj(service), service.Spec.Type)
		return endpointIP, endpointPort, false
	}

	// Retrieve the endpoint address
	if service.Status.LoadBalancer.Ingress[0].IP != "" {
		endpointIP = service.Status.LoadBalancer.Ingress[0].IP
	} else if service.Status.LoadBalancer.Ingress[0].Hostname != "" {
		endpointIP = service.Status.LoadBalancer.Ingress[0].Hostname
	}

	// Retrieve the endpoint port
	for _, port := range service.Spec.Ports {
		if port.Name == wireguard.DriverName {
			endpointPort = strconv.FormatInt(int64(port.Port), 10)
			return endpointIP, endpointPort, true
		}
	}

	klog.Warningf("Port %s not found in service %q", wireguard.DriverName, klog.KObj(service))
	return endpointIP, endpointPort, false
}
