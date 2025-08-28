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

package shadowendpointslicectrl

// This file wires the direct-connections failover (see directconnections.go) into the controller:
// the watch below re-enqueues the involved ShadowEndpointSlices whenever the status of a Connection
// changes, so that Reconcile recomputes the endpoints Ready conditions from the live status.

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// connectionStatusChangedPredicate filters Connection events to the ones that can change the
// failover decision, i.e. the ones affecting the connection status.
func connectionStatusChangedPredicate() predicate.Funcs {
	return predicate.Funcs{
		// Reconcile on create to converge after a controller restart, even if a connection is
		// already down (the informer replays all existing Connections as create events).
		CreateFunc: func(_ event.CreateEvent) bool { return true },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldConn, okOld := e.ObjectOld.(*networkingv1beta1.Connection)
			newConn, okNew := e.ObjectNew.(*networkingv1beta1.Connection)
			if !okOld || !okNew {
				return false
			}
			return oldConn.Status.Value != newConn.Status.Value
		},
		// A deleted Connection means the direct path is gone (the providers unpeered):
		// reconcile so that the affected slices fail over to the indirect path.
		DeleteFunc:  func(_ event.DeleteEvent) bool { return true },
		GenericFunc: func(_ event.GenericEvent) bool { return false },
	}
}

// getConnectionEventHandler maps a Connection event to the ShadowEndpointSlices (both direct
// and indirect companions) whose direct-connections data references the remote cluster
// of that Connection, through the field index registered in SetupWithManager.
func (r *Reconciler) getConnectionEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		shadowList := r.getShadowEndpointSlicesByClusterID(ctx, obj)
		if shadowList == nil {
			return nil
		}

		requests := make([]reconcile.Request, len(shadowList.Items))
		for i := range shadowList.Items {
			requests[i].NamespacedName = types.NamespacedName{
				Name:      shadowList.Items[i].Name,
				Namespace: shadowList.Items[i].Namespace,
			}
		}
		return requests
	})
}
