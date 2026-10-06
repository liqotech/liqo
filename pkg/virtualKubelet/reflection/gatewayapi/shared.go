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
	"context"
	"encoding/json"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
	gwv1apply "sigs.k8s.io/gateway-api/applyconfiguration/apis/v1"

	offloadingv1beta1apply "github.com/liqotech/liqo/pkg/client/applyconfiguration/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// sharedStatusReflector is implemented by the status reflectors of the objects which can be mapped to shared objects
// of the remote cluster, rather than reflected (i.e., the Gateways mapped to the shared Gateway).
type sharedStatusReflector[O object] interface {
	// Shared ensures the shadow resource reports the status of the shared object the given local object is mapped to.
	Shared(ctx context.Context, local O) error
}

var _ sharedStatusReflector[*gwv1.Gateway] = (*gatewayStatusReflector)(nil)

// remoteRoute is a route in the remote namespace, along with its parent references.
type remoteRoute struct {
	metav1.Object
	kind    string
	parents []gwv1.ParentReference
}

// remoteRoutesLister lists the routes of a given kind in the remote namespace.
type remoteRoutesLister func() ([]remoteRoute, error)

// watchSharedGatewayRoutes configures the informers of the reflected routes in the remote namespace, which report the
// addresses of the shared Gateway they are attached to, so that they are reported by the local Gateways mapped to it.
// The Gateways mapped to the shared one are enqueued whenever a remote route changes, to update the addresses.
func watchSharedGatewayRoutes(opts *options.NamespacedOpts, cfg *Config,
	reflector *NamespacedReflector[*gwv1.Gateway, *gwv1apply.GatewayApplyConfiguration]) {
	status, ok := reflector.status.(*gatewayStatusReflector)
	if !ok || !cfg.SharedGatewayEnabled {
		return
	}

	keyer := sharedGatewaysKeyer(reflector.localObjects, &reflector.forgingOpts)
	for _, resource := range cfg.ReflectedRoutes {
		// Starting the remote informer would block the reflection of the whole namespace, as the cache would never sync.
		if !canListAndWatch(opts.RemoteClient, resource, opts.RemoteNamespace) {
			klog.Warningf("Not allowed to list and watch %s in remote namespace %q: the addresses of the shared Gateway cannot be retrieved",
				resource, opts.RemoteNamespace)
			continue
		}

		var informer cache.SharedIndexInformer
		var lister remoteRoutesLister
		switch resource {
		case gwutils.HTTPRoutesGVR.GroupResource():
			informer = opts.RemoteGatewayFactory.Gateway().V1().HTTPRoutes().Informer()
			routes := opts.RemoteGatewayFactory.Gateway().V1().HTTPRoutes().Lister().HTTPRoutes(opts.RemoteNamespace)
			lister = func() ([]remoteRoute, error) {
				objects, err := routes.List(labels.Everything())
				result := make([]remoteRoute, 0, len(objects))
				for _, route := range objects {
					result = append(result, remoteRoute{Object: route, kind: HTTPRouteReflectorName, parents: route.Spec.ParentRefs})
				}
				return result, err
			}
		case gwutils.GRPCRoutesGVR.GroupResource():
			informer = opts.RemoteGatewayFactory.Gateway().V1().GRPCRoutes().Informer()
			routes := opts.RemoteGatewayFactory.Gateway().V1().GRPCRoutes().Lister().GRPCRoutes(opts.RemoteNamespace)
			lister = func() ([]remoteRoute, error) {
				objects, err := routes.List(labels.Everything())
				result := make([]remoteRoute, 0, len(objects))
				for _, route := range objects {
					result = append(result, remoteRoute{Object: route, kind: GRPCRouteReflectorName, parents: route.Spec.ParentRefs})
				}
				return result, err
			}
		default:
			continue
		}

		_, err := informer.AddEventHandler(opts.HandlerFactory(keyer))
		utilruntime.Must(err)
		status.remoteRoutes = append(status.remoteRoutes, lister)
	}
}

// sharedGatewaysKeyer returns a keyer enqueuing the local Gateways mapped to the shared one, in the namespace of the reflector.
func sharedGatewaysKeyer(gateways namespaceLister[*gwv1.Gateway], opts *forge.GatewayAPIForgingOpts) options.Keyer {
	return func(_ metav1.Object) []types.NamespacedName {
		objects, err := gateways.List(labels.Everything())
		utilruntime.Must(err)

		var keys []types.NamespacedName
		for _, gateway := range objects {
			if forge.GatewayMappedToShared(gateway, opts) {
				keys = append(keys, types.NamespacedName{Namespace: gateway.GetNamespace(), Name: gateway.GetName()})
			}
		}
		return keys
	}
}

// Shared ensures the shadow resource reports the status of the shared Gateway the given local Gateway is mapped to.
// The Gateway is programmed once the addresses of the shared Gateway are known, as reported by the remote cluster
// through the reflected routes attached to it, since the shared Gateway itself cannot be accessed.
func (gsr *gatewayStatusReflector) Shared(ctx context.Context, local *gwv1.Gateway) error {
	var previous []metav1.Condition
	var previousListeners []gwv1.ListenerStatus
	if shadow, err := gsr.lister.Get(ShadowName("", local.GetName())); err == nil {
		previous, previousListeners = shadow.Spec.Conditions, shadow.Spec.Listeners
	}

	// The shared Gateway is known only once a route attached to it has been reflected, as resolved by the remote cluster.
	resolved, addresses := gsr.sharedGatewayAddresses()
	shared := "the shared Gateway of the remote cluster"
	if resolved != nil {
		shared = fmt.Sprintf("shared Gateway %q", resolved)
	}

	programmed, reason := metav1.ConditionTrue, string(gwv1.GatewayReasonProgrammed)
	message := fmt.Sprintf("Mapped to %s", shared)
	if len(addresses) == 0 {
		programmed, reason = metav1.ConditionFalse, string(gwv1.GatewayReasonPending)
		message = fmt.Sprintf("Mapped to %s, whose addresses are not yet known (no route attached to it in the namespace, "+
			"or the Liqo version installed in the remote cluster does not report them)", shared)
	}

	conditions := []metav1.Condition{
		newCondition(previous, string(gwv1.GatewayConditionAccepted), metav1.ConditionTrue, string(gwv1.GatewayReasonAccepted),
			fmt.Sprintf("Mapped to %s", shared), local.GetGeneration()),
		newCondition(previous, string(gwv1.GatewayConditionProgrammed), programmed, reason, message, local.GetGeneration()),
	}

	// The listeners of the local Gateway are not applied, as the routes are attached to the shared Gateway regardless of them.
	listeners := make([]gwv1.ListenerStatus, 0, len(local.Spec.Listeners))
	for i := range local.Spec.Listeners {
		name := local.Spec.Listeners[i].Name
		var previousConditions []metav1.Condition
		for j := range previousListeners {
			if previousListeners[j].Name == name {
				previousConditions = previousListeners[j].Conditions
			}
		}

		listenerMessage := fmt.Sprintf("Listener not applied: the attached routes are attached to %s", shared)
		listeners = append(listeners, gwv1.ListenerStatus{
			Name: name,
			SupportedKinds: []gwv1.RouteGroupKind{
				{Group: ptr.To(gwv1.Group(gwv1.GroupName)), Kind: "HTTPRoute"},
				{Group: ptr.To(gwv1.Group(gwv1.GroupName)), Kind: "GRPCRoute"},
			},
			Conditions: []metav1.Condition{
				newCondition(previousConditions, string(gwv1.ListenerConditionAccepted), metav1.ConditionTrue,
					string(gwv1.ListenerReasonAccepted), listenerMessage, local.GetGeneration()),
				newCondition(previousConditions, string(gwv1.ListenerConditionProgrammed), programmed, reason, listenerMessage, local.GetGeneration()),
			},
		})
	}

	return gsr.apply(ctx, local.GetName(), offloadingv1beta1apply.ShadowGatewayStatusSpec().
		WithAddresses(addresses...).
		WithConditions(conditionsApply(conditions)...).
		WithListeners(listeners...))
}

// sharedGatewayAddresses returns the shared Gateway and its addresses, as reported by the reflected routes attached to it
// in the remote namespace, or nil if not known. The routes are considered in a deterministic order.
func (gsr *gatewayStatusReflector) sharedGatewayAddresses() (*types.NamespacedName, []gwv1.GatewayStatusAddress) {
	var routes []remoteRoute
	for _, lister := range gsr.remoteRoutes {
		objects, err := lister()
		utilruntime.Must(err)
		routes = append(routes, objects...)
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].kind != routes[j].kind {
			return routes[i].kind < routes[j].kind
		}
		return routes[i].GetName() < routes[j].GetName()
	})

	for i := range routes {
		route := &routes[i]
		shared := forge.ResolvedSharedGateway(route)
		if shared == nil || !forge.IsReflected(route) || !attachedTo(route, *shared) {
			continue
		}

		value, found := route.GetAnnotations()[consts.SharedGatewayAddressesAnnotation]
		if !found {
			// The shared Gateway is known, although its addresses are not (e.g., it is not yet programmed).
			return shared, nil
		}

		var addresses []gwv1.GatewayStatusAddress
		if err := json.Unmarshal([]byte(value), &addresses); err != nil {
			klog.Warningf("Failed to parse the addresses of the shared Gateway reported by remote %s %q: %v", route.kind, klog.KObj(route), err)
			continue
		}
		return shared, addresses
	}
	return nil, nil
}

// attachedTo returns whether the given remote route is attached to the given Gateway.
func attachedTo(route *remoteRoute, gateway types.NamespacedName) bool {
	for _, parent := range routeParentGateways(route.GetNamespace(), &gwv1.CommonRouteSpec{ParentRefs: route.parents}) {
		if parent == gateway {
			return true
		}
	}
	return false
}
