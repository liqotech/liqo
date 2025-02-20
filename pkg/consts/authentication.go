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

package consts

const (
	// AuthKeysSecretName is the name of the secret containing the authentication keys.
	AuthKeysSecretName = "authentication-keys"

	// SignedNonceSecretLabelKey is the label key used to identify signed nonce secrets.
	SignedNonceSecretLabelKey = "liqo.io/signed-nonce" //nolint:gosec // this is not a credential
	// NonceSecretLabelKey is the key used to store the Nonce value in the Secret.
	NonceSecretLabelKey = "liqo.io/nonce" //nolint:gosec // this is not a credential

	// NonceSecretField is the field key where the nonce is stored in the secret.
	NonceSecretField = "nonce"
	// SignedNonceSecretField is the field key where the signed nonce is stored in the secret.
	SignedNonceSecretField = "signedNonce"

	// KubeconfigSecretField is the field key where the kubeconfig is stored in the secret.
	KubeconfigSecretField = "kubeconfig"

	// IdentityTypeLabelKey is the label key to indicate the type of Identity.
	IdentityTypeLabelKey = "liqo.io/identity-type"

	// RemoteTenantNamespaceAnnotKey is the annotation key used to store the remote tenant namespace.
	RemoteTenantNamespaceAnnotKey = "liqo.io/remote-tenant-namespace"

	// ResourceSliceNameLabelKey is the label key used to store the name of the resource slice.
	ResourceSliceNameLabelKey = "liqo.io/resourceslice-name"

	// CreatorLabelKey is the label key used to store the creator of a resource.
	CreatorLabelKey = "liqo.io/creator-user"

	// CreateVirtualNodeAnnotation is the value of the annotation that enables the creation of a virtual node.
	CreateVirtualNodeAnnotation = "liqo.io/create-virtual-node"

	// CordonResourceAnnotation is the value of the annotation that enables the cordon of a resource.
	CordonResourceAnnotation = "liqo.io/cordon"

	// CordonTenantAnnotation is the value of the annotation that enables the cordon of a tenant.
	CordonTenantAnnotation = "liqo.io/cordon-tenant"

	// RenewAnnotation is the value of the annotation that enables the renewal of a resource.
	RenewAnnotation = "liqo.io/renew"

	// PeeringUserNameLabelKey labels all the resources created to grant peering permissions to the user doing a pering toward this cluster.
	PeeringUserNameLabelKey = "liqo.io/peering-user-name"
)
