// Copyright 2019-2021 The Liqo Authors
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

package liqodeploymentctrl

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// addReconcileRequestForEveryLDP forces the controller to reconcile each LiqoDeployment resource in the cluster.
func addReconcileRequestForEveryLDP(c client.Client, rli workqueue.RateLimitingInterface) {
	ctx := context.Background()
	liqoDeploymentList := offv1alpha1.LiqoDeploymentList{}
	if err := c.List(ctx, &liqoDeploymentList); err != nil {
		klog.Errorf("%s --> Unable to list LiqoDeployment resources.", err)
		return
	}

	// If there are no LiqoDeployment resources in the cluster, no reconcile request will be added.
	for i := range liqoDeploymentList.Items {
		rli.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      liqoDeploymentList.Items[i].GetName(),
				Namespace: liqoDeploymentList.Items[i].GetNamespace(),
			},
		})
	}
}

func selectOnlyVirtualNodes(o client.Object) bool {
	value, ok := (o.GetLabels())[liqoconst.TypeLabel]
	return ok && value == liqoconst.TypeNode
}

// getVirtualNodeEventHandler returns an event handler that reacts on virtual nodes creation, deletion and update.
func getVirtualNodeEventHandler(c client.Client) handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(e event.CreateEvent, rli workqueue.RateLimitingInterface) {
			if selectOnlyVirtualNodes(e.Object) {
				addReconcileRequestForEveryLDP(c, rli)
			}
		},
		UpdateFunc: func(e event.UpdateEvent, rli workqueue.RateLimitingInterface) {
			if selectOnlyVirtualNodes(e.ObjectNew) {
				addReconcileRequestForEveryLDP(c, rli)
			}
		},
		DeleteFunc: func(e event.DeleteEvent, rli workqueue.RateLimitingInterface) {
			if selectOnlyVirtualNodes(e.Object) {
				addReconcileRequestForEveryLDP(c, rli)
			}
		},
		GenericFunc: func(e event.GenericEvent, rli workqueue.RateLimitingInterface) {
			if selectOnlyVirtualNodes(e.Object) {
				addReconcileRequestForEveryLDP(c, rli)
			}
		},
	}
}
