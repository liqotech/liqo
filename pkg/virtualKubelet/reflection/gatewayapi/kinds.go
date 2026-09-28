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

package gatewayapi

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwv1apply "sigs.k8s.io/gateway-api/applyconfiguration/apis/v1"
	gwclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	gwinformers "sigs.k8s.io/gateway-api/pkg/client/informers/externalversions"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	// GatewayReflectorName -> The name associated with the Gateway reflector.
	GatewayReflectorName = "Gateway"
	// HTTPRouteReflectorName -> The name associated with the HTTPRoute reflector.
	HTTPRouteReflectorName = "HTTPRoute"
	// GRPCRouteReflectorName -> The name associated with the GRPCRoute reflector.
	GRPCRouteReflectorName = "GRPCRoute"
	// ReferenceGrantReflectorName -> The name associated with the ReferenceGrant reflector.
	ReferenceGrantReflectorName = "ReferenceGrant"
)

var gatewayKind = kind[*gwv1.Gateway, *gwv1apply.GatewayApplyConfiguration]{
	Name:          GatewayReflectorName,
	GroupResource: gwutils.GatewaysGVR.GroupResource(),
	Informer: func(factory gwinformers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Gateway().V1().Gateways().Informer()
	},
	Lister: func(factory gwinformers.SharedInformerFactory, namespace string) namespaceLister[*gwv1.Gateway] {
		return factory.Gateway().V1().Gateways().Lister().Gateways(namespace)
	},
	Client: func(client gwclient.Interface, namespace string) namespaceClient[*gwv1apply.GatewayApplyConfiguration, *gwv1.Gateway] {
		return client.GatewayV1().Gateways(namespace)
	},
	Forge:           forge.RemoteGateway,
	StatusReflector: newGatewayStatusReflector,
}

var httpRouteKind = kind[*gwv1.HTTPRoute, *gwv1apply.HTTPRouteApplyConfiguration]{
	Name:          HTTPRouteReflectorName,
	GroupResource: gwutils.HTTPRoutesGVR.GroupResource(),
	Informer: func(factory gwinformers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Gateway().V1().HTTPRoutes().Informer()
	},
	Lister: func(factory gwinformers.SharedInformerFactory, namespace string) namespaceLister[*gwv1.HTTPRoute] {
		return factory.Gateway().V1().HTTPRoutes().Lister().HTTPRoutes(namespace)
	},
	Client: func(client gwclient.Interface, namespace string) namespaceClient[*gwv1apply.HTTPRouteApplyConfiguration, *gwv1.HTTPRoute] {
		return client.GatewayV1().HTTPRoutes(namespace)
	},
	Forge: forge.RemoteHTTPRoute,
	ParentGateways: func(route *gwv1.HTTPRoute) []types.NamespacedName {
		return routeParentGateways(route.GetNamespace(), &route.Spec.CommonRouteSpec)
	},
	StatusReflector: newRouteStatusReflector(offloadingv1beta1.HTTPRouteKind,
		func(route *gwv1.HTTPRoute) []gwv1.ParentReference { return route.Spec.ParentRefs },
		func(route *gwv1.HTTPRoute) []gwv1.RouteParentStatus { return route.Status.Parents }),
}

var grpcRouteKind = kind[*gwv1.GRPCRoute, *gwv1apply.GRPCRouteApplyConfiguration]{
	Name:          GRPCRouteReflectorName,
	GroupResource: gwutils.GRPCRoutesGVR.GroupResource(),
	Informer: func(factory gwinformers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Gateway().V1().GRPCRoutes().Informer()
	},
	Lister: func(factory gwinformers.SharedInformerFactory, namespace string) namespaceLister[*gwv1.GRPCRoute] {
		return factory.Gateway().V1().GRPCRoutes().Lister().GRPCRoutes(namespace)
	},
	Client: func(client gwclient.Interface, namespace string) namespaceClient[*gwv1apply.GRPCRouteApplyConfiguration, *gwv1.GRPCRoute] {
		return client.GatewayV1().GRPCRoutes(namespace)
	},
	Forge: forge.RemoteGRPCRoute,
	ParentGateways: func(route *gwv1.GRPCRoute) []types.NamespacedName {
		return routeParentGateways(route.GetNamespace(), &route.Spec.CommonRouteSpec)
	},
	StatusReflector: newRouteStatusReflector(offloadingv1beta1.GRPCRouteKind,
		func(route *gwv1.GRPCRoute) []gwv1.ParentReference { return route.Spec.ParentRefs },
		func(route *gwv1.GRPCRoute) []gwv1.RouteParentStatus { return route.Status.Parents }),
}

var referenceGrantKind = kind[*gwv1.ReferenceGrant, *gwv1apply.ReferenceGrantApplyConfiguration]{
	Name:          ReferenceGrantReflectorName,
	GroupResource: gwutils.ReferenceGrantsGVR.GroupResource(),
	Informer: func(factory gwinformers.SharedInformerFactory) cache.SharedIndexInformer {
		return factory.Gateway().V1().ReferenceGrants().Informer()
	},
	Lister: func(factory gwinformers.SharedInformerFactory, namespace string) namespaceLister[*gwv1.ReferenceGrant] {
		return factory.Gateway().V1().ReferenceGrants().Lister().ReferenceGrants(namespace)
	},
	Client: func(client gwclient.Interface, namespace string) namespaceClient[*gwv1apply.ReferenceGrantApplyConfiguration, *gwv1.ReferenceGrant] {
		return client.GatewayV1().ReferenceGrants(namespace)
	},
	Forge: forge.RemoteReferenceGrant,
}

// NewGatewayReflector returns a new Gateway reflector.
func NewGatewayReflector(reflectorConfig *offloadingv1beta1.ReflectorConfig, cfg *Config) manager.Reflector {
	return newReflector(&gatewayKind, reflectorConfig, cfg)
}

// NewHTTPRouteReflector returns a new HTTPRoute reflector.
func NewHTTPRouteReflector(reflectorConfig *offloadingv1beta1.ReflectorConfig, cfg *Config) manager.Reflector {
	return newReflector(&httpRouteKind, reflectorConfig, cfg)
}

// NewGRPCRouteReflector returns a new GRPCRoute reflector.
func NewGRPCRouteReflector(reflectorConfig *offloadingv1beta1.ReflectorConfig, cfg *Config) manager.Reflector {
	return newReflector(&grpcRouteKind, reflectorConfig, cfg)
}

// NewReferenceGrantReflector returns a new ReferenceGrant reflector.
func NewReferenceGrantReflector(reflectorConfig *offloadingv1beta1.ReflectorConfig, cfg *Config) manager.Reflector {
	return newReflector(&referenceGrantKind, reflectorConfig, cfg)
}

// NewNamespacedGatewayReflector returns a function generating NamespacedReflector instances for Gateways.
func NewNamespacedGatewayReflector(cfg *Config) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return newNamespacedReflector(&gatewayKind, cfg)
}

// NewNamespacedHTTPRouteReflector returns a function generating NamespacedReflector instances for HTTPRoutes.
func NewNamespacedHTTPRouteReflector(cfg *Config) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return newNamespacedReflector(&httpRouteKind, cfg)
}

// NewNamespacedGRPCRouteReflector returns a function generating NamespacedReflector instances for GRPCRoutes.
func NewNamespacedGRPCRouteReflector(cfg *Config) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return newNamespacedReflector(&grpcRouteKind, cfg)
}

// NewNamespacedReferenceGrantReflector returns a function generating NamespacedReflector instances for ReferenceGrants.
func NewNamespacedReferenceGrantReflector(cfg *Config) func(*options.NamespacedOpts) manager.NamespacedReflector {
	return newNamespacedReflector(&referenceGrantKind, cfg)
}

// NewClusterFallback returns the function registering the event handlers on the cluster-wide local informers for the
// given type of reflector, which are then used by the namespaced reflectors. It is exposed for testing purposes.
func NewClusterFallback(reflectorName string, cfg *Config) func(*options.ReflectorOpts) manager.FallbackReflector {
	switch reflectorName {
	case GatewayReflectorName:
		return newClusterFallback(&gatewayKind, cfg)
	case HTTPRouteReflectorName:
		return newClusterFallback(&httpRouteKind, cfg)
	case GRPCRouteReflectorName:
		return newClusterFallback(&grpcRouteKind, cfg)
	case ReferenceGrantReflectorName:
		return newClusterFallback(&referenceGrantKind, cfg)
	default:
		return nil
	}
}
