// Copyright 2019-2021 The Liqo Authors
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

	"github.com/liqotech/liqo/pkg/auth"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
)

// IdentityReader provides the interface to retrieve the identities for the remote clusters.
type IdentityReader interface {
	GetConfig(remoteClusterID string, namespace string) (*rest.Config, error)
	GetRemoteTenantNamespace(remoteClusterID string, namespace string) (string, error)
}

// IdentityManager interface provides the methods to manage identities for the remote clusters.
type IdentityManager interface {
	IdentityReader

	CreateIdentity(remoteClusterID string) (*v1.Secret, error)
	GetSigningRequest(remoteClusterID string) ([]byte, error)
	StoreCertificate(remoteClusterID string, identityResponse *auth.CertificateIdentityResponse) error
}

// IdentityProvider provides the interface to retrieve and approve remote cluster identities.
type IdentityProvider interface {
	GetRemoteCertificate(clusterID, namespace, signingRequest string) (response *responsetypes.SigningRequestResponse, err error)
	ApproveSigningRequest(clusterID, signingRequest string) (response *responsetypes.SigningRequestResponse, err error)
}
