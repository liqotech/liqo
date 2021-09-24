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

package resourcerequestoperator

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// getForeignClusterEventHandler returns an event handler that reacts on ForeignClusters updates.
// In particular, it reacts on changes over the incomingPeering flag triggering the reconciliation
// of the related ResourceRequest.
func getForeignClusterEventHandler(c client.Client) handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(ce event.CreateEvent, rli workqueue.RateLimitingInterface) {},
		UpdateFunc: func(ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
			ctx := context.Background()

			oldForeignCluster, ok := ue.ObjectOld.(*discoveryv1alpha1.ForeignCluster)
			if !ok {
				klog.Errorf("object %v is not a ForeignCluster", ue.ObjectOld)
				return
			}

			newForeignCluster, ok := ue.ObjectNew.(*discoveryv1alpha1.ForeignCluster)
			if !ok {
				klog.Errorf("object %v is not a ForeignCluster", ue.ObjectNew)
				return
			}

			remoteClusterID := newForeignCluster.Spec.ClusterIdentity.ClusterID
			if oldForeignCluster.Spec.IncomingPeeringEnabled != newForeignCluster.Spec.IncomingPeeringEnabled {
				var resourceRequestList discoveryv1alpha1.ResourceRequestList
				if err := c.List(ctx, &resourceRequestList, client.HasLabels{
					consts.ReplicationStatusLabel}, client.MatchingLabels{
					consts.ReplicationOriginLabel: remoteClusterID,
				}); err != nil {
					klog.Error(err)
					return
				}

				switch len(resourceRequestList.Items) {
				case 0:
					klog.V(3).Infof("no ResourceRequest found for ID %v", remoteClusterID)
					return
				case 1:
					resourceRequest := &resourceRequestList.Items[0]
					rli.Add(reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      resourceRequest.GetName(),
							Namespace: resourceRequest.GetNamespace(),
						},
					})
					return
				default:
					klog.Warningf("multiple ResourceRequest found for ID %v", remoteClusterID)
					return
				}
			}
		},
		DeleteFunc:  func(de event.DeleteEvent, rli workqueue.RateLimitingInterface) {},
		GenericFunc: func(ge event.GenericEvent, rli workqueue.RateLimitingInterface) {},
	}
}
