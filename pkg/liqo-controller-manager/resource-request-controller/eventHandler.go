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
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
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
					crdreplicator.ReplicationStatuslabel}, client.MatchingLabels{
					crdreplicator.RemoteLabelSelector: remoteClusterID,
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
