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

package neighborhood

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

func GetNeighborhoodForCluster(ctx context.Context, cl client.Client, clusterID string) (*discoveryv1alpha1.Neighborhood, error) {
	var neighborhoodList discoveryv1alpha1.NeighborhoodList
	// Get all the resources with remoteID label set to clusterID
	req, err := labels.NewRequirement(consts.ReplicationDestinationLabel, selection.Equals, []string{clusterID})
	if err != nil {
		return nil, err
	}
	if err := cl.List(ctx, &neighborhoodList, &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(*req),
	}); err != nil {
		return nil, err
	}

	if len(neighborhoodList.Items) == 0 {
		return nil, kerrors.NewNotFound(discoveryv1alpha1.NeighborhoodGroupResource, "")
	}

	if len(neighborhoodList.Items) > 1 {
		// CHECK Should never happen
		klog.Warning("multiple neighborhood resources found for cluster %s", clusterID)
		if err := DeleteNeighborhoodsForCluster(ctx, cl, clusterID); err != nil {
			klog.Error(err)
			return nil, err
		}
		return nil, kerrors.NewNotFound(discoveryv1alpha1.NeighborhoodGroupResource, "")
	}

	return &neighborhoodList.Items[0], nil
}

func DeleteNeighborhoodsForCluster(ctx context.Context, cl client.Client, clusterID string) error {
	var neighborhoodList discoveryv1alpha1.NeighborhoodList
	req, err := labels.NewRequirement(consts.ReplicationDestinationLabel, selection.Equals, []string{clusterID})
	if err != nil {
		return err
	}
	if err := cl.List(ctx, &neighborhoodList, &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(*req),
	}); err != nil {
		return err
	}

	for _, neighborhood := range neighborhoodList.Items {
		if err := cl.Delete(ctx, &neighborhood); err != nil {
			klog.Warningf("Error while deleting Neighborhood %s for cluster %s: %v", neighborhood.Name, clusterID, err)
			return err
		}
	}

	klog.Infof("Deleted all neighborhood resources for cluster %s", clusterID)
	return nil
}
