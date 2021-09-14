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

package neighborhoodcreator

import (
	"context"
	"reflect"
	"strconv"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	discoveryPkg "github.com/liqotech/liqo/pkg/discovery"
)

const neighborhoodPrefix = "neighborhood-"

type NeighborhoodCreator struct {
	client.Client
	Scheme    *runtime.Scheme
	ClusterID string
}

var (
	result = ctrl.Result{
		Requeue:      true,
		RequeueAfter: 10 * time.Second,
	}
)

const (
	neighborhoodLabelKey   = "liqo/neighborhood"
	neighborhoodLabelValue = "true"
)

// clusterRole
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=neighborhoods,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles ForeignCluster resources.
// For each FC it ensures a Neighborhood resource exists and is updated.
func (r *NeighborhoodCreator) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var foreignCluster discoveryv1alpha1.ForeignCluster
	if err := r.Get(ctx, req.NamespacedName, &foreignCluster); err != nil {
		return result, client.IgnoreNotFound(err)
	}
	if foreignCluster.Spec.ClusterIdentity.ClusterID == "" || foreignCluster.Status.TenantNamespace.Local == "" {
		return result, nil
	}
	if err := r.ensureNeighborhoodForCluster(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return result, err
	}
	return result, nil
}

func (r *NeighborhoodCreator) ensureNeighborhoodForCluster(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster) error {
	neighbors, err := r.getExistingNeighbors(ctx)
	if err != nil {
		klog.Errorf("unable to get existing neighbors: %w", err)
		return err
	}
	// Get neighborhood resource for cluster.
	neighborhoodRes, err := r.getNeighborhoodPerCluster(ctx, fc.Spec.ClusterIdentity.ClusterID)
	if client.IgnoreNotFound(err) != nil {
		klog.Error(err)
		return err
	}

	// Remove the ForeignCluster ClusterID from the list.
	delete(neighbors, fc.Spec.ClusterIdentity.ClusterID)

	// Create the resource if not already present (if the error is not nil, then at this point is a not found one)
	if err != nil {
		return r.createNeighborhood(ctx, fc, neighbors)
	}

	// Otherwise, update the resource to ensure it is up-to-date
	return r.updateNeighborhood(ctx, neighborhoodRes, neighbors)
}

func (r *NeighborhoodCreator) getNeighborhoodPerCluster(ctx context.Context, clusterID string) (*discoveryv1alpha1.Neighborhood, error) {
	var neighborhoodsList discoveryv1alpha1.NeighborhoodList
	// Get all the resource with remoteID label set to clusterID
	requirement, err := labels.NewRequirement(consts.ReplicationDestinationLabel, selection.Equals, []string{clusterID})
	if err != nil {
		return nil, err
	}
	remoteIDSelector := labels.NewSelector().Add(*requirement)
	if err := r.List(ctx, &neighborhoodsList, &client.ListOptions{
		LabelSelector: remoteIDSelector,
	}); err != nil {
		return nil, err
	}
	if len(neighborhoodsList.Items) != 1 {
		if len(neighborhoodsList.Items) == 0 {
			return nil, kerrors.NewNotFound(discoveryv1alpha1.NeighborhoodGroupResource, "")
		}
		klog.Warning("multiple neighborhood resources found for cluster %s", clusterID)
		if err := r.deleteMultipleNeighborhood(ctx, clusterID); err != nil {
			klog.Error(err)
			return nil, err
		}
		return nil, kerrors.NewNotFound(discoveryv1alpha1.NeighborhoodGroupResource, "")
	}
	return &neighborhoodsList.Items[0], nil
}

func (r *NeighborhoodCreator) deleteMultipleNeighborhood(ctx context.Context, clusterID string) error {
	neighborhoodRequirement, err := labels.NewRequirement(neighborhoodLabelKey, selection.Equals, []string{neighborhoodLabelValue})
	if err != nil {
		return err
	}
	clusterIDRequirement, err := labels.NewRequirement(consts.ReplicationDestinationLabel, selection.Equals, []string{clusterID})
	if err != nil {
		return err
	}
	if err := r.DeleteAllOf(ctx, nil, &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			LabelSelector: labels.NewSelector().Add(*neighborhoodRequirement, *clusterIDRequirement),
		},
	}); err != nil {
		return err
	}
	klog.Infof("Deleted all neighborhood resources for cluster %s", clusterID)
	return nil
}
func (r *NeighborhoodCreator) createNeighborhood(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster, existingNeighbors map[string]discoveryv1alpha1.Neighbor) error {
	neighborhood := forgeNeighborhoodResource(r.ClusterID, fc, existingNeighbors)
	if err := r.Create(ctx, neighborhood); err != nil {
		return err
	}
	klog.Infof("Resource %s for cluster %s correctly created.", neighborhood.GetName(), fc.Spec.ClusterIdentity.ClusterID)
	return nil
}

func (r *NeighborhoodCreator) updateNeighborhood(ctx context.Context, neighborhoodRes *discoveryv1alpha1.Neighborhood, neighbors map[string]discoveryv1alpha1.Neighbor) error {
	if reflect.DeepEqual(neighborhoodRes.Spec.NeighborsList, neighbors) {
		return nil
	}
	neighborhoodRes.Spec.NeighborsList = neighbors
	if err := r.Update(ctx, neighborhoodRes, &client.UpdateOptions{}); err != nil {
		return err
	}
	klog.Infof("Resource %s correctly updated", neighborhoodRes.GetName())
	return nil
}

func forgeNeighborhoodResource(localClusterID string, fc *discoveryv1alpha1.ForeignCluster, neighbors map[string]discoveryv1alpha1.Neighbor) *discoveryv1alpha1.Neighborhood {
	return &discoveryv1alpha1.Neighborhood{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: neighborhoodPrefix,
			Namespace:    fc.Status.TenantNamespace.Local,
			Labels: map[string]string{
				consts.ReplicationDestinationLabel: fc.Spec.ClusterIdentity.ClusterID,
				consts.ReplicationRequestedLabel:   strconv.FormatBool(true),
			},
		},
		Spec: discoveryv1alpha1.NeighborhoodSpec{
			ClusterID:     localClusterID,
			NeighborsList: neighbors,
		},
		Status: discoveryv1alpha1.NeighborhoodStatus{},
	}
}

func (r *NeighborhoodCreator) getExistingNeighbors(ctx context.Context) (map[string]discoveryv1alpha1.Neighbor, error) {
	var foreignClusterList discoveryv1alpha1.ForeignClusterList
	if err := r.List(ctx, &foreignClusterList, &client.ListOptions{}); err != nil {
		return nil, err
	}
	neighborsList := make(map[string]discoveryv1alpha1.Neighbor, len(foreignClusterList.Items))
	for _, fc := range foreignClusterList.Items {
		neighborID := fc.Spec.ClusterIdentity.ClusterID
		neighborsList[neighborID] = discoveryv1alpha1.Neighbor{}
	}
	return neighborsList, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NeighborhoodCreator) SetupWithManager(mgr ctrl.Manager) error {
	filterInduced, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      discoveryPkg.DiscoveryTypeLabel,
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{string(discoveryPkg.InducedPeeringDiscovery)},
			},
		},
	})
	if err != nil {
		klog.Error(err)
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ForeignCluster{}, builder.WithPredicates(filterInduced)).
		Complete(r)
}
