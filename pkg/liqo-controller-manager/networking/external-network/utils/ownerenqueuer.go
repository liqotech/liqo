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

package utils

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NewOwnerEnqueuer returns a new OwnerEnqueuer.
func NewOwnerEnqueuer(ownerKind string) handler.EventHandler {
	return &OwnerEnqueuer{
		ownerKind: ownerKind,
	}
}

var _ handler.EventHandler = &OwnerEnqueuer{}

// OwnerEnqueuer is an event handler that enqueues the owner of the object for a given kind.
type OwnerEnqueuer struct {
	ownerKind string
}

// Create enqueues the owner of the object for a given kind.
func (h *OwnerEnqueuer) Create(_ context.Context, _ event.CreateEvent, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	panic("implement me")
}

// Update enqueues the owner of the object for a given kind.
func (h *OwnerEnqueuer) Update(_ context.Context, _ event.UpdateEvent, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	panic("implement me")
}

// Delete enqueues the owner of the object for a given kind.
func (h *OwnerEnqueuer) Delete(_ context.Context, _ event.DeleteEvent, _ workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	panic("implement me")
}

// Generic enqueues the owner of the object for a given kind.
func (h *OwnerEnqueuer) Generic(_ context.Context, evt event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	owners := evt.Object.GetOwnerReferences()

	for _, owner := range owners {
		if owner.Kind == h.ownerKind {
			q.Add(reconcile.Request{
				NamespacedName: client.ObjectKey{
					Namespace: evt.Object.GetNamespace(),
					Name:      owner.Name,
				},
			})
			return
		}
	}
}
