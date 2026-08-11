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

package route

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// NewRouteBindingWatchSource creates a new Source for the RouteConfigurationBinding watcher.
func NewRouteBindingWatchSource(src <-chan event.GenericEvent, eh handler.EventHandler) source.Source {
	return source.Channel(src, eh)
}

// NewRouteCfgBindingEnqueuer creates a new EventHandler for RouteConfigurationBinding resources.
// It enqueues all bindings whose spec.targetRef matches the given target.
func NewRouteCfgBindingEnqueuer(cl client.Client, target networkingv1beta1.RouteBindingTargetReference) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			list := &networkingv1beta1.RouteConfigurationBindingList{}
			if err := cl.List(ctx, list); err != nil {
				klog.Error(err)
				return nil
			}
			var requests []reconcile.Request
			for i := range list.Items {
				if !MatchesRouteTargetRef(&list.Items[i].Spec.TargetRef, target) {
					continue
				}
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
					Name:      list.Items[i].Name,
					Namespace: list.Items[i].Namespace,
				}})
			}
			return requests
		})
}

// NewRouteCfgToRouteCfgBindingEnqueuer returns an EventHandler that,
// given a RouteConfiguration object, enqueues all RouteConfigurationBindings in
// the same namespace that reference it through spec.routeConfigurationRef.name and
// match the given target.
func NewRouteCfgToRouteCfgBindingEnqueuer(cl client.Client, target networkingv1beta1.RouteBindingTargetReference) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			routecfg, ok := obj.(*networkingv1beta1.RouteConfiguration)
			if !ok || routecfg == nil {
				return nil
			}

			bindingList := &networkingv1beta1.RouteConfigurationBindingList{}
			if err := cl.List(ctx, bindingList, client.InNamespace(routecfg.Namespace)); err != nil {
				klog.Error(err)
				return nil
			}

			var requests []reconcile.Request
			for i := range bindingList.Items {
				binding := &bindingList.Items[i]
				if binding.Spec.RouteConfigurationRef.Name == routecfg.Name &&
					MatchesRouteTargetRef(&binding.Spec.TargetRef, target) {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      binding.Name,
						Namespace: binding.Namespace,
					}})
				}
			}
			return requests
		})
}

// ForgeRouteTargetRefPredicate returns a predicate that matches RouteConfigurationBinding resources
// whose spec.targetRef equals the given target.
func ForgeRouteTargetRefPredicate(target networkingv1beta1.RouteBindingTargetReference) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		binding, ok := obj.(*networkingv1beta1.RouteConfigurationBinding)
		if !ok {
			return false
		}
		return MatchesRouteTargetRef(&binding.Spec.TargetRef, target)
	})
}

// MatchesRouteTargetRef returns true if the given RouteBindingTargetReference matches the provided target.
func MatchesRouteTargetRef(ref *networkingv1beta1.RouteBindingTargetReference, target networkingv1beta1.RouteBindingTargetReference) bool {
	return ref.APIVersion == target.APIVersion &&
		ref.Kind == target.Kind &&
		ref.Name == target.Name &&
		ref.Namespace == target.Namespace
}
