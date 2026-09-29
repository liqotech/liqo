// Copyright 2019-2026 The Liqo Authors
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

package util

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/liqotech/liqo/pkg/utils/resource"
)

const (
	// EnvoyGatewayNamespace is the namespace where Envoy Gateway deploys the proxies serving the Gateways.
	EnvoyGatewayNamespace = "envoy-gateway-system"
	// GatewayListenerName is the name of the HTTP listener of the Gateways created by EnforceGateway.
	GatewayListenerName = "http"
)

// EnforceGateway creates or updates a Gateway of the given class, with a single HTTP listener on port 80.
func EnforceGateway(ctx context.Context, cl client.Client, namespace, name, class string) error {
	gateway := &gwv1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	return Second(resource.CreateOrUpdate(ctx, cl, gateway, func() error {
		gateway.Spec.GatewayClassName = gwv1.ObjectName(class)
		gateway.Spec.Listeners = []gwv1.Listener{{Name: GatewayListenerName, Protocol: gwv1.HTTPProtocolType, Port: 80}}
		return nil
	}))
}

// HTTPRouteOption is a function that modifies an HTTPRoute.
type HTTPRouteOption func(*gwv1.HTTPRoute)

// WithRouteLabels sets the given labels on the HTTPRoute.
func WithRouteLabels(labels map[string]string) HTTPRouteOption {
	return func(route *gwv1.HTTPRoute) { route.SetLabels(labels) }
}

// WithExtensionRefFilter adds an implementation-specific filter to the rule of the HTTPRoute.
func WithExtensionRefFilter() HTTPRouteOption {
	return func(route *gwv1.HTTPRoute) {
		route.Spec.Rules[0].Filters = []gwv1.HTTPRouteFilter{{Type: gwv1.HTTPRouteFilterExtensionRef,
			ExtensionRef: &gwv1.LocalObjectReference{Group: "e2e.liqo.io", Kind: "Auth", Name: "auth"}}}
	}
}

// EnforceHTTPRoute creates or updates an HTTPRoute, attached to the given parents and forwarding the requests
// for the given hostname to port 80 of the given backend Service.
func EnforceHTTPRoute(ctx context.Context, cl client.Client, namespace, name string, parents []gwv1.ParentReference,
	hostname, backend string, options ...HTTPRouteOption) error {
	route := &gwv1.HTTPRoute{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	return Second(resource.CreateOrUpdate(ctx, cl, route, func() error {
		route.Spec.ParentRefs = parents
		route.Spec.Hostnames = []gwv1.Hostname{gwv1.Hostname(hostname)}
		route.Spec.Rules = []gwv1.HTTPRouteRule{{BackendRefs: []gwv1.HTTPBackendRef{{BackendRef: gwv1.BackendRef{
			BackendObjectReference: gwv1.BackendObjectReference{Name: gwv1.ObjectName(backend), Port: ptr.To[gwv1.PortNumber](80)},
		}}}}}
		for _, option := range options {
			option(route)
		}
		return nil
	}))
}

// EnforceTLSRoute creates or updates a TLSRoute, attached to the given parents and forwarding the connections
// for the given hostname to port 443 of the given backend Service.
func EnforceTLSRoute(ctx context.Context, cl client.Client, namespace, name string, parents []gwv1.ParentReference,
	hostname, backend string) error {
	route := &gwv1.TLSRoute{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	return Second(resource.CreateOrUpdate(ctx, cl, route, func() error {
		route.Spec.ParentRefs = parents
		route.Spec.Hostnames = []gwv1.Hostname{gwv1.Hostname(hostname)}
		route.Spec.Rules = []gwv1.TLSRouteRule{{BackendRefs: []gwv1.BackendRef{{
			BackendObjectReference: gwv1.BackendObjectReference{Name: gwv1.ObjectName(backend), Port: ptr.To[gwv1.PortNumber](443)},
		}}}}
		return nil
	}))
}

// EnvoyProxyTarget returns the name of a running Envoy pod serving the given Gateway, and the container port
// corresponding to the given port of the Gateway, which differ since Envoy does not bind privileged ports.
func EnvoyProxyTarget(ctx context.Context, cl client.Client, gatewayNamespace, gatewayName string,
	port int32) (pod string, targetPort int, err error) {
	selector := client.MatchingLabels{
		"gateway.envoyproxy.io/owning-gateway-name":      gatewayName,
		"gateway.envoyproxy.io/owning-gateway-namespace": gatewayNamespace,
	}

	var services corev1.ServiceList
	if err := cl.List(ctx, &services, client.InNamespace(EnvoyGatewayNamespace), selector); err != nil {
		return "", 0, err
	}
	for i := range services.Items {
		for _, p := range services.Items[i].Spec.Ports {
			if p.Port == port {
				targetPort = p.TargetPort.IntValue()
			}
		}
	}
	if targetPort == 0 {
		return "", 0, fmt.Errorf("no Envoy service exposing port %d for Gateway %s/%s", port, gatewayNamespace, gatewayName)
	}

	var pods corev1.PodList
	if err := cl.List(ctx, &pods, client.InNamespace(EnvoyGatewayNamespace), selector); err != nil {
		return "", 0, err
	}
	for i := range pods.Items {
		if pods.Items[i].Status.Phase == corev1.PodRunning && pods.Items[i].DeletionTimestamp == nil {
			return pods.Items[i].Name, targetPort, nil
		}
	}
	return "", 0, fmt.Errorf("no running Envoy pod for Gateway %s/%s", gatewayNamespace, gatewayName)
}
