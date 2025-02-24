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

	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
)

// GenerateTenant generates a Tenant resource to be applied on a remote cluster.
// Using the cluster keys it generates a CSR to obtain a ControlPlane Identity from
// the provider cluster.
// It needs the local cluster identity to get the authentication keys and the signature
// of the nonce given by the provider cluster to complete the authentication challenge.
func GenerateTenant(ctx context.Context, cl client.Client,
	localClusterID liqov1beta1.ClusterID, liqoNamespace, remoteTenantNamespace string,
	signature []byte, proxyURL *string) (*authv1beta1.Tenant, error) {
	// Get public and private keys of the local cluster.
	privateKey, publicKey, err := authentication.GetClusterKeys(ctx, cl, liqoNamespace)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster keys: %w", err)
	}

	// Generate a CSR for the remote cluster.
	CSR, err := authentication.GenerateCSRForControlPlane(privateKey, localClusterID)
	if err != nil {
		return nil, fmt.Errorf("unable to generate CSR: %w", err)
	}

	// Forge tenant resource for the remote cluster.
	return forge.TenantForRemoteCluster(localClusterID, publicKey, CSR, signature, &remoteTenantNamespace, proxyURL), nil
}
