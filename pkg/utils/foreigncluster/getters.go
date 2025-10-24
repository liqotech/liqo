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

package foreigncluster

import (
	"context"
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// GetForeignClusterByID returns a ForeignCluster CR retrieving it by its clusterID.
//
// This function implements a three-tier fallback lookup strategy to support both
// standard label-based peering and out-of-band (manual) peering scenarios:
//
// 1. Label-based lookup (O(1) with index): Searches for ForeignClusters with the
//    liqo.io/remote-cluster-id label matching the clusterID. This is the standard
//    path used by liqoctl peer and is highly efficient.
//
// 2. Name-based lookup (O(1)): Fallback for out-of-band peering where ForeignCluster
//    resources are created manually (e.g., via GitOps, kubectl apply) with name == clusterID.
//    Common in RKE2 deployments and restricted network environments.
//
// 3. Exhaustive search (O(n)): Final fallback that iterates through ALL ForeignClusters
//    to find one with spec.ClusterID matching the requested ID. This is expensive and
//    should rarely be triggered in production.
//
// Performance considerations:
//   - In clusters with many ForeignClusters (>100), the exhaustive search can impact
//     API server performance. Consider adding liqo.io/remote-cluster-id labels to
//     manually-created ForeignClusters to avoid this fallback.
//   - The function logs when fallback #2 or #3 is used to aid in debugging and
//     identifying misconfigured resources.
func GetForeignClusterByID(ctx context.Context, cl client.Client, clusterID liqov1beta1.ClusterID) (*liqov1beta1.ForeignCluster, error) {
	// Fallback #1: Label-based lookup (most efficient, O(1) with index)
	lSelector := labels.SelectorFromSet(labels.Set{
		consts.RemoteClusterID: string(clusterID),
	})
	foreignClusterList := liqov1beta1.ForeignClusterList{}
	if err := cl.List(ctx, &foreignClusterList, &client.ListOptions{
		LabelSelector: lSelector,
	}); err != nil {
		return nil, err
	}

	// If found by label, return immediately (fast path)
	if len(foreignClusterList.Items) > 0 {
		klog.V(4).Infof("Found ForeignCluster %s by label lookup (fast path)", clusterID)
		return getForeignCluster(&foreignClusterList, clusterID)
	}

	// Fallback #2: Name-based lookup for out-of-band peering (O(1))
	// This supports manually-created ForeignCluster resources where name == clusterID
	klog.V(4).Infof("Label lookup failed for ForeignCluster %s, trying name-based lookup (out-of-band peering)", clusterID)
	fc := &liqov1beta1.ForeignCluster{}
	err := cl.Get(ctx, client.ObjectKey{Name: string(clusterID)}, fc)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Fallback #3: Exhaustive search through all ForeignClusters (O(n))
			// WARNING: This is expensive and can impact performance in large clusters
			klog.Warningf("Name-based lookup failed for ForeignCluster %s, performing exhaustive search across all ForeignClusters (expensive operation)", clusterID)
			allFCs := &liqov1beta1.ForeignClusterList{}
			if listErr := cl.List(ctx, allFCs); listErr == nil {
				for i := range allFCs.Items {
					if allFCs.Items[i].Spec.ClusterID == clusterID {
						klog.Warningf("Found ForeignCluster %s via exhaustive search. Consider adding the %s label to this resource for better performance",
							clusterID, consts.RemoteClusterID)
						return &allFCs.Items[i], nil
					}
				}
			}
		}
		return nil, kerrors.NewNotFound(liqov1beta1.ForeignClusterGroupResource, fmt.Sprintf("foreign cluster with ID %s", clusterID))
	}

	// Validate that the ForeignCluster found by name has the correct spec.ClusterID
	if fc.Spec.ClusterID != "" && fc.Spec.ClusterID != clusterID {
		klog.Warningf("ForeignCluster %s found by name but spec.ClusterID mismatch (expected: %s, got: %s)", fc.Name, clusterID, fc.Spec.ClusterID)
		return nil, kerrors.NewNotFound(liqov1beta1.ForeignClusterGroupResource, fmt.Sprintf("foreign cluster with ID %s", clusterID))
	}

	klog.V(4).Infof("Found ForeignCluster %s by name-based lookup (out-of-band peering)", clusterID)
	return fc, nil
}

// GetForeignClusterByIDWithDynamicClient returns a ForeignCluster CR retrieving it by its clusterID, using the dynamic interface.
func GetForeignClusterByIDWithDynamicClient(ctx context.Context, dynClient dynamic.Interface, clusterID liqov1beta1.ClusterID) (
	*liqov1beta1.ForeignCluster, error) {
	lSelector := labels.SelectorFromSet(labels.Set{
		consts.RemoteClusterID: string(clusterID),
	})
	unstr, err := dynClient.Resource(liqov1beta1.ForeignClusterGroupVersionResource).List(ctx, metav1.ListOptions{
		LabelSelector: lSelector.String()})
	if err != nil {
		return nil, err
	}

	foreignClusterList := liqov1beta1.ForeignClusterList{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &foreignClusterList)
	if err != nil {
		return nil, err
	}

	return getForeignCluster(&foreignClusterList, clusterID)
}

func getForeignCluster(foreignClusterList *liqov1beta1.ForeignClusterList,
	clusterID liqov1beta1.ClusterID) (*liqov1beta1.ForeignCluster, error) {
	switch len(foreignClusterList.Items) {
	case 0:
		return nil, kerrors.NewNotFound(liqov1beta1.ForeignClusterGroupResource, fmt.Sprintf("foreign cluster with ID %s", clusterID))
	case 1:
		return &foreignClusterList.Items[0], nil
	default:
		return GetOlderForeignCluster(foreignClusterList), nil
	}
}

// GetOlderForeignCluster returns the ForeignCluster from the list with the older creationTimestamp.
func GetOlderForeignCluster(
	foreignClusterList *liqov1beta1.ForeignClusterList) (foreignCluster *liqov1beta1.ForeignCluster) {
	var olderTime *metav1.Time
	for i := range foreignClusterList.Items {
		fc := &foreignClusterList.Items[i]
		if olderTime == nil || fc.CreationTimestamp.Before(olderTime) {
			olderTime = &fc.CreationTimestamp
			foreignCluster = fc
		}
	}
	return foreignCluster
}

// GetLocalTenantNamespaceName gets the name of the local tenant namespace associated with a specific peering (remoteClusterID).
func GetLocalTenantNamespaceName(ctx context.Context, cl client.Client, remoteCluster liqov1beta1.ClusterID) (string, error) {
	fc, err := GetForeignClusterByID(ctx, cl, remoteCluster)
	if err != nil {
		klog.Errorf("%s -> unable to get foreignCluster associated with the cluster '%s'", err, remoteCluster)
		return "", err
	}

	if fc.Status.TenantNamespace.Local == "" {
		err = fmt.Errorf("there is no tenant namespace associated with the peering with the remote cluster '%s'",
			remoteCluster)
		klog.Error(err)
		return "", err
	}
	return fc.Status.TenantNamespace.Local, nil
}

// GetRemoteTenantNamespaceName gets the name of the remote tenant namespace associated with a specific peering (remoteClusterID).
func GetRemoteTenantNamespaceName(ctx context.Context, cl client.Client, remoteClusterID liqov1beta1.ClusterID) (string, error) {
	fc, err := GetForeignClusterByID(ctx, cl, remoteClusterID)
	if err != nil {
		klog.Errorf("%s -> unable to get foreignCluster associated with the clusterID '%s'", err, remoteClusterID)
		return "", err
	}

	if fc.Status.TenantNamespace.Remote == "" {
		err = fmt.Errorf("there is no tenant namespace associated with the peering with the remote cluster '%s'",
			remoteClusterID)
		klog.Error(err)
		return "", err
	}
	return fc.Status.TenantNamespace.Remote, nil
}
