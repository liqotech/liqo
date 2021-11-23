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

package auth

import (
	"encoding/base64"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// IdentityRequest is the common interface for Certificate and ServiceAccount identity request.
type IdentityRequest interface {
	GetClusterIdentity() discoveryv1alpha1.ClusterIdentity
	GetToken() string
	GetPath() string
}

// ServiceAccountIdentityRequest is the request for a new ServiceAccount validation.
type ServiceAccountIdentityRequest struct {
	ClusterIdentity discoveryv1alpha1.ClusterIdentity `json:"cluster"`
	Token           string                            `json:"token"`
}

// CertificateIdentityRequest is the request for a new certificate validation.
type CertificateIdentityRequest struct {
	ClusterIdentity discoveryv1alpha1.ClusterIdentity `json:"cluster"`
	// OriginClusterToken will be used by the remote cluster to obtain an identity to send us its ResourceOffers
	// and NetworkConfigs.
	OriginClusterToken        string `json:"originClusterToken,omitempty"`
	DestinationClusterToken   string `json:"destinationClusterToken"`
	CertificateSigningRequest string `json:"certificateSigningRequest"`
}

// NewCertificateIdentityRequest creates and returns a new CertificateIdentityRequest.
func NewCertificateIdentityRequest(cluster discoveryv1alpha1.ClusterIdentity, originClusterToken, token string,
	certificateSigningRequest []byte) *CertificateIdentityRequest {
	return &CertificateIdentityRequest{
		ClusterIdentity:           cluster,
		OriginClusterToken:        originClusterToken,
		DestinationClusterToken:   token,
		CertificateSigningRequest: base64.StdEncoding.EncodeToString(certificateSigningRequest),
	}
}

// GetClusterIdentity returns the ClusterIdentity.
func (saIdentityRequest *ServiceAccountIdentityRequest) GetClusterIdentity() discoveryv1alpha1.ClusterIdentity {
	return saIdentityRequest.ClusterIdentity
}

// GetToken returns the token.
func (saIdentityRequest *ServiceAccountIdentityRequest) GetToken() string {
	return saIdentityRequest.Token
}

// GetPath returns the absolute path of the endpoint to contact to send a new ServiceAccountIdentityRequest.
func (saIdentityRequest *ServiceAccountIdentityRequest) GetPath() string {
	return IdentityURI
}

// GetClusterIdentity returns the ClusterIdentity.
func (certIdentityRequest *CertificateIdentityRequest) GetClusterIdentity() discoveryv1alpha1.ClusterIdentity {
	return certIdentityRequest.ClusterIdentity
}

// GetToken returns the token.
func (certIdentityRequest *CertificateIdentityRequest) GetToken() string {
	return certIdentityRequest.DestinationClusterToken
}

// GetPath returns the absolute path of the endpoint to contact to send a new CertificateIdentityRequest.
func (certIdentityRequest *CertificateIdentityRequest) GetPath() string {
	return CertIdentityURI
}
