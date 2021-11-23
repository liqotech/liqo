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

package foreigncluster

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// GetLocalTenantNamespaceName gets the name of the local tenant namespace associated with a specific peering (remoteClusterID).
func GetLocalTenantNamespaceName(ctx context.Context, cl client.Client, remoteCluster discoveryv1alpha1.ClusterIdentity) (string, error) {
	fc, err := GetForeignClusterByID(ctx, cl, remoteCluster.ClusterID)
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
func GetRemoteTenantNamespaceName(ctx context.Context, cl client.Client, remoteClusterID string) (string, error) {
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
