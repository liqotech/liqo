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

package sharedgatewayctrl

import (
	"context"
	"encoding/json"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// RouteReconciler annotates the routes of a given kind reflected from the consumer clusters, and attached to the shared
// Gateways offered by the local cluster, with the addresses of the latter. The consumer clusters are not allowed to access
// the shared Gateways, while they can read the routes reflected in their namespaces, and report the addresses in the status
// of their Gateways mapped to the shared ones.
type RouteReconciler struct {
	client.Client

	// Kind is the kind of routes managed by the reconciler.
	Kind offloadingv1beta1.RouteKind
	// SharedGateways are the Gateways offered by the local cluster to the consumer clusters.
	SharedGateways []types.NamespacedName
}

// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes;grpcroutes,verbs=get;list;watch;patch

// Reconcile ensures the given route reports the addresses of the shared Gateway it is attached to, if any.
func (r *RouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	route, parents := r.newRoute()
	if err := r.Get(ctx, req.NamespacedName, route); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	desired, err := r.sharedGatewayAddresses(ctx, route.GetNamespace(), *parents)
	if err != nil {
		return ctrl.Result{}, err
	}

	current, found := route.GetAnnotations()[consts.SharedGatewayAddressesAnnotation]
	if (desired == "" && !found) || (desired != "" && current == desired) {
		return ctrl.Result{}, nil
	}

	original := route.DeepCopyObject().(client.Object)
	annotations := route.GetAnnotations()
	if desired == "" {
		delete(annotations, consts.SharedGatewayAddressesAnnotation)
	} else {
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[consts.SharedGatewayAddressesAnnotation] = desired
	}
	route.SetAnnotations(annotations)

	if err := r.Patch(ctx, route, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to annotate %s %q with the addresses of the shared Gateway: %w", r.Kind, req.NamespacedName, err)
	}
	klog.V(4).Infof("%s %q annotated with the addresses of the shared Gateway: %q", r.Kind, req.NamespacedName, desired)
	return ctrl.Result{}, nil
}

// sharedGatewayAddresses returns the JSON-encoded addresses of the first shared Gateway the route is attached to, which is
// programmed, or an empty string if none. The addresses are reported only once programmed, as they might be otherwise not ready.
func (r *RouteReconciler) sharedGatewayAddresses(ctx context.Context, namespace string, parents []gwv1.ParentReference) (string, error) {
	for _, shared := range r.attachedSharedGateways(namespace, parents) {
		var gateway gwv1.Gateway
		if err := r.Get(ctx, shared, &gateway); err != nil {
			if kerrors.IsNotFound(err) {
				continue
			}
			return "", fmt.Errorf("failed to retrieve shared Gateway %q: %w", shared, err)
		}

		if !meta.IsStatusConditionTrue(gateway.Status.Conditions, string(gwv1.GatewayConditionProgrammed)) || len(gateway.Status.Addresses) == 0 {
			continue
		}

		addresses, err := json.Marshal(gateway.Status.Addresses)
		if err != nil {
			return "", fmt.Errorf("failed to encode the addresses of shared Gateway %q: %w", shared, err)
		}
		return string(addresses), nil
	}
	return "", nil
}

// attachedSharedGateways returns the shared Gateways the route with the given parents is attached to, in order.
func (r *RouteReconciler) attachedSharedGateways(namespace string, parents []gwv1.ParentReference) []types.NamespacedName {
	var attached []types.NamespacedName
	for i := range parents {
		parent := &parents[i]
		if (parent.Group != nil && *parent.Group != gwv1.GroupName) || (parent.Kind != nil && *parent.Kind != "Gateway") {
			continue
		}

		gateway := types.NamespacedName{Namespace: namespace, Name: string(parent.Name)}
		if parent.Namespace != nil {
			gateway.Namespace = string(*parent.Namespace)
		}
		if r.isShared(gateway) {
			attached = append(attached, gateway)
		}
	}
	return attached
}

// isShared returns whether the given Gateway is offered as shared Gateway to the consumer clusters.
func (r *RouteReconciler) isShared(gateway types.NamespacedName) bool {
	for _, shared := range r.SharedGateways {
		if shared == gateway {
			return true
		}
	}
	return false
}

func (r *RouteReconciler) newRoute() (client.Object, *[]gwv1.ParentReference) {
	switch r.Kind {
	case offloadingv1beta1.GRPCRouteKind:
		route := &gwv1.GRPCRoute{}
		return route, &route.Spec.ParentRefs
	default:
		route := &gwv1.HTTPRoute{}
		return route, &route.Spec.ParentRefs
	}
}

func (r *RouteReconciler) newRouteList() client.ObjectList {
	switch r.Kind {
	case offloadingv1beta1.GRPCRouteKind:
		return &gwv1.GRPCRouteList{}
	default:
		return &gwv1.HTTPRouteList{}
	}
}

// SetupWithManager registers the controller, which watches the reflected routes, and the shared Gateways they are attached to.
func (r *RouteReconciler) SetupWithManager(mgr ctrl.Manager, workers int) error {
	route, _ := r.newRoute()
	name := consts.CtrlSharedGatewayHTTPRoute
	if r.Kind == offloadingv1beta1.GRPCRouteKind {
		name = consts.CtrlSharedGatewayGRPCRoute
	}

	// Only the routes reflected from the consumer clusters are annotated.
	reflected := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, found := obj.GetLabels()[forge.LiqoOriginClusterIDKey]
		return found
	})
	shared := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return r.isShared(types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()})
	})

	return ctrl.NewControllerManagedBy(mgr).Named(name).
		For(route, builder.WithPredicates(reflected)).
		Watches(&gwv1.Gateway{}, handler.EnqueueRequestsFromMapFunc(r.attachedRoutes), builder.WithPredicates(shared)).
		WithOptions(controller.Options{MaxConcurrentReconciles: workers}).
		Complete(r)
}

// attachedRoutes returns the requests for the reflected routes attached to the given shared Gateway.
func (r *RouteReconciler) attachedRoutes(ctx context.Context, gateway client.Object) []reconcile.Request {
	routes := r.newRouteList()
	if err := r.List(ctx, routes, client.HasLabels{forge.LiqoOriginClusterIDKey}); err != nil {
		klog.Errorf("Failed to list the %ss attached to shared Gateway %q: %v", r.Kind, klog.KObj(gateway), err)
		return nil
	}

	key := types.NamespacedName{Namespace: gateway.GetNamespace(), Name: gateway.GetName()}
	var requests []reconcile.Request
	err := meta.EachListItem(routes, func(obj runtime.Object) error {
		route, ok := obj.(client.Object)
		if !ok {
			return nil
		}
		var parents []gwv1.ParentReference
		switch typed := route.(type) {
		case *gwv1.HTTPRoute:
			parents = typed.Spec.ParentRefs
		case *gwv1.GRPCRoute:
			parents = typed.Spec.ParentRefs
		}
		for _, attached := range r.attachedSharedGateways(route.GetNamespace(), parents) {
			if attached == key {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: route.GetNamespace(), Name: route.GetName()}})
				break
			}
		}
		return nil
	})
	if err != nil {
		klog.Errorf("Failed to process the %ss attached to shared Gateway %q: %v", r.Kind, klog.KObj(gateway), err)
	}
	return requests
}
