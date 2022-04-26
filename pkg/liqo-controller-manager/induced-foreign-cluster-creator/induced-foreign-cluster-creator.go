/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package inducedforeignclustercreator

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	discovery "github.com/liqotech/liqo/pkg/discoverymanager"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// InducedForeignClusterCreator reconciles a Neighborhood object
type InducedForeignClusterCreator struct {
	client.Client
	Scheme        *runtime.Scheme
	DiscoveryCtrl *discovery.Controller
}

var (
	result = ctrl.Result{}
)

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Neighborhood object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *InducedForeignClusterCreator) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("Reconciling Neighborhood %q", req)

	var neighborhood discoveryv1alpha1.Neighborhood
	if err := r.Get(ctx, req.NamespacedName, &neighborhood); err != nil {
		klog.Infof("Neighborhood %q not found in %s", req, req.NamespacedName)
		return result, client.IgnoreNotFound(err)
	}
	clusterID := neighborhood.Spec.ClusterID
	klog.Infof("Neighborhood %s sender is %s", req, clusterID)
	// Get ForeignCluster resource relative to the sender of the neighborhood resource.
	_, err := foreignclusterutils.GetForeignClusterByID(context.Background(), r.Client, clusterID)
	if apierrors.IsNotFound(err) {
		klog.Errorf("neighborhood resource sender %s not found.", clusterID)
		return result, err
	}
	// Ensure it exists an Induced ForeignCluster for each neighbor in the resource.
	if err := r.DiscoveryCtrl.UpdateInducedForeignClusters(ctx, r.Client, clusterID, neighborhood.Spec.Neighbors); err != nil {
		klog.Errorf("unable to update Induced ForeignClusters from neighbors of %s: %w", clusterID, err)
		return result, err
	}
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InducedForeignClusterCreator) SetupWithManager(mgr ctrl.Manager) error {
	// The InducedForeignClusterCreator has to process only neighborhoods resources received by other clusters.
	filterLocalNeighborhoods, err := predicate.LabelSelectorPredicate(reflection.RemoteResourcesLabelSelector())
	if err != nil {
		klog.Error(err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.Neighborhood{}, builder.WithPredicates(filterLocalNeighborhoods)).
		Complete(r)
}
