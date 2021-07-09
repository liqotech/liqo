package foreignclusteroperator

import (
	"context"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// validateForeignCluster contains the logic that validates and defaults labels and spec fields.
// TODO: this function will be refactored in a future pr.
func (r *ForeignClusterReconciler) validateForeignCluster(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (cont bool, res ctrl.Result, err error) {
	requireUpdate := false

	if r.needsClusterIdentityDefaulting(foreignCluster) {
		// this ForeignCluster has not all the required fields, probably it has been added manually, so default to exposed values
		if err := r.clusterIdentityDefaulting(foreignCluster); err != nil {
			klog.Error(err)
			return false, ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		// the resource has been updated, no need to requeue
		return false, ctrl.Result{}, nil
	}

	// set cluster-id label to easy retrieve ForeignClusters by ClusterId,
	// if it is added manually, the name maybe not coincide with ClusterId
	if foreignCluster.ObjectMeta.Labels[discovery.ClusterIDLabel] == "" {
		if foreignCluster.ObjectMeta.Labels == nil {
			foreignCluster.ObjectMeta.Labels = map[string]string{}
		}
		foreignCluster.ObjectMeta.Labels[discovery.ClusterIDLabel] = foreignCluster.Spec.ClusterIdentity.ClusterID
		requireUpdate = true
	}

	if requireUpdate {
		_, err := r.update(foreignCluster)
		if err != nil {
			klog.Error(err, err.Error())
			return false, ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
		return false, ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, nil
	}

	return true, ctrl.Result{}, nil
}
