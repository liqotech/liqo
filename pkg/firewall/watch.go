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

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// NewFirewallWatchSource creates a new Source for the Firewall watcher.
func NewFirewallWatchSource(src <-chan event.GenericEvent, eh handler.EventHandler) source.Source {
	return source.Channel(src, eh)
}

// NewFirewallWatchEventHandler creates a new EventHandler.
func NewFirewallWatchEventHandler(cl client.Client, labelsSets []labels.Set) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			var requests []reconcile.Request
			for k := range labelsSets {
				list, err := getters.ListFirewallConfigurationsByLabel(ctx, cl, labels.SelectorFromSet(labelsSets[k]))
				if err != nil {
					klog.Error(err)
					return nil
				}
				for i := range list.Items {
					item := &list.Items[i]
					requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: item.Name, Namespace: item.Namespace}})
				}
			}

			return requests
		})
}

// NewFirewallConfigurationBindingByConfigurationHandler returns an EventHandler that,
// given a FirewallConfiguration object, enqueues all FirewallConfigurationBindings in
// the same namespace that reference it through spec.firewallConfigurationRef.name and
// match the given target (apiVersion, kind, name, namespace).
func NewFirewallConfigurationBindingByConfigurationHandler(cl client.Client, apiVersion, kind, name, namespace string) handler.EventHandler {
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
					MatchesTargetRef(&binding.Spec.TargetRef, apiVersion, kind, name, namespace) {
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

// NewFirewallBindingWatchEventHandler creates a new EventHandler for FirewallConfigurationBinding resources.
// It enqueues all bindings whose spec.targetRef matches the given apiVersion, kind, name and namespace.
func NewFirewallBindingWatchEventHandler(cl client.Client, apiVersion, kind, name, namespace string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			list := &networkingv1beta1.FirewallConfigurationBindingList{}
			if err := cl.List(ctx, list); err != nil {
				klog.Error(err)
				return nil
			}
			var requests []reconcile.Request
			for i := range list.Items {
				if !MatchesTargetRef(&list.Items[i].Spec.TargetRef, apiVersion, kind, name, namespace) {
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
