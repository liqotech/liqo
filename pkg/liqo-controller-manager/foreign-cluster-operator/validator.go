// Copyright 2019-2022 The Liqo Authors
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

package foreignclusteroperator

import (
	"context"
	"fmt"
	"net/url"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

// validateForeignCluster contains the logic that validates and defaults labels and spec fields.
func (r *ForeignClusterReconciler) validateForeignCluster(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (cont bool, res ctrl.Result, err error) {
	requireUpdate := false

	if r.needsClusterIdentityDefaulting(foreignCluster) {
		// this ForeignCluster has not all the cluster identity fields (clusterID and clusterName),
		// get them from the foreignAuthUrl.
		if err := r.clusterIdentityDefaulting(ctx, foreignCluster); err != nil {
			klog.Error(err)
			return false, ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.ResyncPeriod,
			}, err
		}
		requireUpdate = true
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
		if err = r.Client.Update(ctx, foreignCluster); err != nil {
			klog.Error(err, err.Error())
			return false, ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.ResyncPeriod,
			}, err
		}
		klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
		return false, ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.ResyncPeriod,
		}, nil
	}

	return true, ctrl.Result{}, nil
}

// isClusterProcessable checks if the provided ForeignCluster is processable.
// It can not be processable if:
// * the clusterID is the same of the local cluster;
// * the same clusterID is already present in a previously created ForeignCluster
// * the specified foreign proxy URL is invalid, if set to a value different that empty string.
func (r *ForeignClusterReconciler) isClusterProcessable(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (bool, error) {
	foreignClusterID := foreignCluster.Spec.ClusterIdentity.ClusterID

	if foreignClusterID == r.HomeCluster.ClusterID {
		// this is the local cluster, it is not processable
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.ProcessForeignClusterStatusCondition,
			discoveryv1alpha1.PeeringConditionStatusError,
			"LocalCluster",
			"This cluster has the same clusterID of the local cluster",
		)

		return false, nil
	}

	foreignClusterWithSameID, err := foreignclusterutils.GetForeignClusterByID(ctx,
		r.Client, foreignClusterID)
	if err != nil {
		klog.Error(err)
		return false, err
	}

	// these are the same resource, no clusterID repetition
	if foreignClusterWithSameID.GetUID() == foreignCluster.GetUID() {
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.ProcessForeignClusterStatusCondition,
			discoveryv1alpha1.PeeringConditionStatusSuccess,
			"ForeignClusterProcesssable",
			"This ForeignCluster seems to be processable",
		)

		return true, nil
	}

	_, err = url.Parse(foreignCluster.Spec.ForeignProxyURL)
	if err != nil {
		peeringconditionsutils.EnsureStatus(foreignCluster,
			discoveryv1alpha1.ProcessForeignClusterStatusCondition,
			discoveryv1alpha1.PeeringConditionStatusError,
			"InvalidProxyURL",
			fmt.Sprintf("Invalid Proxy URL %s: (%v)", foreignCluster.Spec.ForeignProxyURL, err),
		)
		return false, nil
	}

	peeringconditionsutils.EnsureStatus(foreignCluster,
		discoveryv1alpha1.ProcessForeignClusterStatusCondition,
		discoveryv1alpha1.PeeringConditionStatusError,
		"ClusterIDRepetition",
		fmt.Sprintf("The same clusterID is already present in another ForeignCluster (%v)", foreignClusterWithSameID.GetName()),
	)
	return false, nil
}
