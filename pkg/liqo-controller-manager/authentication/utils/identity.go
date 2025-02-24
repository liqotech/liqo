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

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// GenerateIdentityControlPlane generates an Identity resource of type ControlPlane to be
// applied on the consumer cluster.
func GenerateIdentityControlPlane(ctx context.Context, cl client.Client,
	remoteClusterID liqov1beta1.ClusterID, remoteTenantNamespace string,
	localClusterID liqov1beta1.ClusterID, localTenantNamespace *string) (*authv1beta1.Identity, error) {
	// Get tenant with the given remote clusterID.
	tenant, err := getters.GetTenantByClusterID(ctx, cl, remoteClusterID, ptr.Deref(localTenantNamespace, corev1.NamespaceAll))
	if err != nil {
		return nil, fmt.Errorf("an error occurred while retrieving tenant: %w", err)
	}

	// Check if the tenant has the required status fields.
	if tenant.Status.AuthParams == nil || tenant.Status.TenantNamespace == "" {
		return nil, fmt.Errorf("tenant %s does not have the required status fields", tenant.Name)
	}

	// Forge Identity resource for the remote cluster and output it.
	authParams := tenant.Status.AuthParams
	identity := forge.IdentityForRemoteCluster(forge.ControlPlaneIdentityName(localClusterID), remoteTenantNamespace,
		localClusterID, authv1beta1.ControlPlaneIdentityType, authParams, &tenant.Status.TenantNamespace)

	return identity, nil
}
