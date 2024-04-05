// Copyright 2019-2024 The Liqo Authors
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

package forge

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// TenantForRemoteCluster forges a Tenant resource to be applied on a remote cluster.
func TenantForRemoteCluster(localClusterIdentity discoveryv1alpha1.ClusterIdentity,
	publicKey, csr, signature []byte, proxyURL *string) *authv1alpha1.Tenant {
	tenant := Tenant(localClusterIdentity)
	MutateTenant(tenant, localClusterIdentity, publicKey, csr, signature, proxyURL)

	return tenant
}

// Tenant forges a Tenant resource.
func Tenant(remoteClusterIdentity discoveryv1alpha1.ClusterIdentity) *authv1alpha1.Tenant {
	return &authv1alpha1.Tenant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: authv1alpha1.GroupVersion.String(),
			Kind:       authv1alpha1.TenantKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: remoteClusterIdentity.ClusterName,
		},
	}
}

// MutateTenant mutates a Tenant resource.
func MutateTenant(tenant *authv1alpha1.Tenant, remoteClusterIdentity discoveryv1alpha1.ClusterIdentity,
	publicKey, csr, signature []byte, proxyURL *string) {
	if tenant.Labels == nil {
		tenant.Labels = map[string]string{}
	}
	tenant.Labels[consts.RemoteClusterID] = remoteClusterIdentity.ClusterID

	tenant.Spec = authv1alpha1.TenantSpec{
		ClusterIdentity: remoteClusterIdentity,
		PublicKey:       publicKey,
		CSR:             csr,
		Signature:       signature,
		ProxyURL:        proxyURL,
	}
}
