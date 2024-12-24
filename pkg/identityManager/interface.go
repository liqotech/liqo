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

package identitymanager

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/auth"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
)

// IdentityReader provides the interface to retrieve the identities for the remote clusters.
type IdentityReader interface {
	GetConfig(remoteCluster liqov1beta1.ClusterID, namespace string) (*rest.Config, error)
	GetConfigFromSecret(remoteCluster liqov1beta1.ClusterID, secret *corev1.Secret) (*rest.Config, error)
	GetRemoteTenantNamespace(remoteCluster liqov1beta1.ClusterID, namespace string) (string, error)
	GetSecretNamespacedName(remoteCluster liqov1beta1.ClusterID, namespace string) (types.NamespacedName, error)
}

// IdentityManager interface provides the methods to manage identities for the remote clusters.
type IdentityManager interface {
	IdentityReader

	StoreIdentity(ctx context.Context, remoteCluster liqov1beta1.ClusterID, namespace string, key []byte,
		remoteProxyURL string, identityResponse *auth.CertificateIdentityResponse) error
}

// SigningRequestOptions contains the options to handle a signing request.
type SigningRequestOptions struct {
	Cluster         liqov1beta1.ClusterID
	TenantNamespace string
	IdentityType    authv1beta1.IdentityType
	Name            string
	SigningRequest  []byte

	// optional
	APIServerAddressOverride string
	CAOverride               []byte
	TrustedCA                bool
	ResourceSlice            *authv1beta1.ResourceSlice
	ProxyURL                 *string
	IsUpdate                 bool
}

// IdentityProvider provides the interface to retrieve and approve remote cluster identities.
type IdentityProvider interface {
	// deprecated
	GetRemoteCertificate(ctx context.Context, options *SigningRequestOptions) (response *responsetypes.SigningRequestResponse, err error)
	// deprecated
	ApproveSigningRequest(ctx context.Context, options *SigningRequestOptions) (response *responsetypes.SigningRequestResponse, err error)
	ForgeAuthParams(ctx context.Context, options *SigningRequestOptions) (*authv1beta1.AuthParams, error)
}

var _ IdentityProvider = &certificateIdentityProvider{}
var _ IdentityProvider = &iamIdentityProvider{}
