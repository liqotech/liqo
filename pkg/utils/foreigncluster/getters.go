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
func GetForeignClusterByID(ctx context.Context, cl client.Client, clusterID liqov1beta1.ClusterID) (*liqov1beta1.ForeignCluster, error) {
	lSelector := labels.SelectorFromSet(labels.Set{
		consts.RemoteClusterID: string(clusterID),
	})
	// get the foreign cluster by clusterID label
	foreignClusterList := liqov1beta1.ForeignClusterList{}
	if err := cl.List(ctx, &foreignClusterList, &client.ListOptions{
		LabelSelector: lSelector,
	}); err != nil {
		return nil, err
	}

	return getForeignCluster(&foreignClusterList, clusterID)
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
		if olderTime.IsZero() || fc.CreationTimestamp.Before(olderTime) {
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
