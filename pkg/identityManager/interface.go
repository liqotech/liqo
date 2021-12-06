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

package identitymanager

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
)

// IdentityReader provides the interface to retrieve the identities for the remote clusters.
type IdentityReader interface {
	GetConfig(remoteCluster discoveryv1alpha1.ClusterIdentity, namespace string) (*rest.Config, error)
	GetRemoteTenantNamespace(remoteCluster discoveryv1alpha1.ClusterIdentity, namespace string) (string, error)
}

// IdentityManager interface provides the methods to manage identities for the remote clusters.
type IdentityManager interface {
	IdentityReader

	CreateIdentity(remoteCluster discoveryv1alpha1.ClusterIdentity) (*v1.Secret, error)
	GetSigningRequest(remoteCluster discoveryv1alpha1.ClusterIdentity) ([]byte, error)
	StoreCertificate(remoteCluster discoveryv1alpha1.ClusterIdentity, remoteProxyURL string, identityResponse *auth.CertificateIdentityResponse) error
}

// IdentityProvider provides the interface to retrieve and approve remote cluster identities.
type IdentityProvider interface {
	GetRemoteCertificate(cluster discoveryv1alpha1.ClusterIdentity,
		namespace, signingRequest string) (response *responsetypes.SigningRequestResponse, err error)
	ApproveSigningRequest(cluster discoveryv1alpha1.ClusterIdentity,
		signingRequest string) (response *responsetypes.SigningRequestResponse, err error)
}
