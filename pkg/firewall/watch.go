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

package firewall

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// NewFwCfgToFwCfgBindingEnqueuer returns an EventHandler that,
// given a FirewallConfiguration object, enqueues all FirewallConfigurationBindings in
// the same namespace that reference it through spec.firewallConfigurationRef.name and
// match the given target.
func NewFwCfgToFwCfgBindingEnqueuer(cl client.Client, target networkingv1beta1.TargetReference) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			fwcfg, ok := obj.(*networkingv1beta1.FirewallConfiguration)
			if !ok || fwcfg == nil {
				return nil
			}

			bindingList := &networkingv1beta1.FirewallConfigurationBindingList{}
			if err := cl.List(ctx, bindingList, client.InNamespace(fwcfg.Namespace)); err != nil {
				klog.Error(err)
				return nil
			}

			var requests []reconcile.Request
			for i := range bindingList.Items {
				binding := &bindingList.Items[i]
				if binding.Spec.FirewallConfigurationRef.Name == fwcfg.Name &&
					MatchesTargetRef(&binding.Spec.TargetRef, target) {
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      binding.Name,
						Namespace: binding.Namespace,
					}})
				}
			}
			return requests
		})
}

// NewFirewallBindingWatchSource creates a new Source for the FirewallConfigurationBinding watcher.
func NewFirewallBindingWatchSource(src <-chan event.GenericEvent, eh handler.EventHandler) source.Source {
	return source.Channel(src, eh)
}

// NewFwCfgBindingEnqueuer creates a new EventHandler for FirewallConfigurationBinding resources.
// It enqueues all bindings whose spec.targetRef matches the given target.
func NewFwCfgBindingEnqueuer(cl client.Client, target networkingv1beta1.TargetReference) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			list := &networkingv1beta1.FirewallConfigurationBindingList{}
			if err := cl.List(ctx, list); err != nil {
				klog.Error(err)
				return nil
			}
			var requests []reconcile.Request
			for i := range list.Items {
				if !MatchesTargetRef(&list.Items[i].Spec.TargetRef, target) {
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
