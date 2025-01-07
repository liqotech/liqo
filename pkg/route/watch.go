// Copyright 2019-2025 The Liqo Authors
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

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/liqotech/liqo/pkg/utils/getters"
)

// NewRouteWatchSource creates a new Source for the Route watcher.
func NewRouteWatchSource(src <-chan event.GenericEvent, eh handler.EventHandler) source.Source {
	return source.Channel(src, eh)
}

// NewRouteWatchEventHandler creates a new EventHandler.
func NewRouteWatchEventHandler(cl client.Client, labelsSets []labels.Set) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			var requests []reconcile.Request
			for k := range labelsSets {
				list, err := getters.ListRouteConfigurationsByLabel(ctx, cl, labels.SelectorFromSet(labelsSets[k]))
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
