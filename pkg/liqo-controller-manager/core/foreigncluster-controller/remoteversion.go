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

package foreignclustercontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	versionpkg "github.com/liqotech/liqo/pkg/liqo-controller-manager/version"
)

// handleRemoteVersion attempts to fetch the remote cluster's Liqo version
// and update it in the ForeignCluster status.
// It handles both consumer and provider scenarios:
// - Consumer: Uses Identity credentials to fetch provider version
// - Provider: Uses Tenant's consumer token to fetch consumer version
func (r *ForeignClusterReconciler) handleRemoteVersion(ctx context.Context, fc *liqov1beta1.ForeignCluster) {
	clusterID := fc.Spec.ClusterID

	// Try consumer approach first: use Identity credentials
	remoteVersion := r.getRemoteVersionAsConsumer(ctx, clusterID)

	// If consumer approach didn't work, try provider approach: use Tenant credentials
	if remoteVersion == "" {
		remoteVersion = r.getRemoteVersionAsProvider(ctx, clusterID)
	}

	// Update the ForeignCluster status if version changed
	if remoteVersion != "" && remoteVersion != fc.Status.RemoteVersion {
		klog.Infof("Updated remote version for ForeignCluster %q: %s", clusterID, remoteVersion)
	}

	fc.Status.RemoteVersion = remoteVersion
}

// getRemoteVersionAsConsumer fetches the provider's version using Identity credentials.
// This is used when the local cluster is a consumer.
func (r *ForeignClusterReconciler) getRemoteVersionAsConsumer(ctx context.Context, clusterID liqov1beta1.ClusterID) string {
	if r.IdentityManager == nil {
		klog.V(6).Infof("IdentityManager not available, skipping consumer version fetch for cluster %q", clusterID)
		return ""
	}

	// Try to get a config for the remote cluster using the identity manager.
	// We use corev1.NamespaceAll to search across all tenant namespaces.
	config, err := r.IdentityManager.GetConfig(clusterID, corev1.NamespaceAll)
	if err != nil {
		// If we can't get a config, it means we don't have credentials for this cluster yet.
		// This is expected during the initial peering phase or for provider clusters.
		klog.V(6).Infof("Unable to get Identity config for remote cluster %q: %v", clusterID, err)
		return ""
	}

	// Create a clientset to access the remote cluster.
	remoteClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.V(4).Infof("Failed to create clientset from Identity for remote cluster %q: %v", clusterID, err)
		return ""
	}

	// Fetch the remote version.
	return versionpkg.GetRemoteVersion(ctx, remoteClientset, r.LiqoNamespace)
}

// getRemoteVersionAsProvider fetches the consumer's version using Tenant's version reader token.
// This is used when the local cluster is a provider.
func (r *ForeignClusterReconciler) getRemoteVersionAsProvider(ctx context.Context, clusterID liqov1beta1.ClusterID) string {
	// Get the Tenant resource for this cluster
	tenant, err := r.getTenantByClusterID(ctx, clusterID)
	if err != nil {
		klog.V(6).Infof("Unable to get Tenant for cluster %q: %v", clusterID, err)
		return ""
	}

	// Check if the Tenant has the consumer's API server URL and token
	if tenant.Spec.ConsumerAPIServerURL == "" || tenant.Spec.ConsumerVersionReaderToken == "" {
		klog.V(6).Infof("Tenant for cluster %q does not have consumer API server URL or version reader token", clusterID)
		return ""
	}

	// Fetch the remote version using the token
	return versionpkg.GetRemoteVersionWithToken(ctx, tenant.Spec.ConsumerAPIServerURL, tenant.Spec.ConsumerVersionReaderToken, r.LiqoNamespace)
}

// getTenantByClusterID retrieves the Tenant resource for the given cluster ID.
func (r *ForeignClusterReconciler) getTenantByClusterID(ctx context.Context, clusterID liqov1beta1.ClusterID) (*authv1beta1.Tenant, error) {
	var tenantList authv1beta1.TenantList
	if err := r.List(ctx, &tenantList); err != nil {
		return nil, fmt.Errorf("failed to list Tenants: %w", err)
	}

	for i := range tenantList.Items {
		if tenantList.Items[i].Spec.ClusterID == clusterID {
			return &tenantList.Items[i], nil
		}
	}

	return nil, fmt.Errorf("tenant not found for cluster %q", clusterID)
}
