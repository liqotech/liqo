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

package auth

import "encoding/base64"

// IdentityRequest is the common interface for Certificate and ServiceAccount identity request.
type IdentityRequest interface {
	GetClusterID() string
	GetToken() string
	GetPath() string
}

// ServiceAccountIdentityRequest is the request for a new ServiceAccount validation.
type ServiceAccountIdentityRequest struct {
	ClusterID string `json:"clusterID"`
	Token     string `json:"token"`
}

// CertificateIdentityRequest is the request for a new certificate validation.
type CertificateIdentityRequest struct {
	ClusterID string `json:"clusterID"`
	// OriginClusterToken will be used by the remote cluster to obtain an identity to send us its ResourceOffers
	// and NetworkConfigs.
	OriginClusterToken        string `json:"originClusterToken,omitempty"`
	DestinationClusterToken   string `json:"destinationClusterToken"`
	CertificateSigningRequest string `json:"certificateSigningRequest"`
}

// NewCertificateIdentityRequest creates and returns a new CertificateIdentityRequest.
func NewCertificateIdentityRequest(clusterID, originClusterToken, token string,
	certificateSigningRequest []byte) *CertificateIdentityRequest {
	return &CertificateIdentityRequest{
		ClusterID:                 clusterID,
		OriginClusterToken:        originClusterToken,
		DestinationClusterToken:   token,
		CertificateSigningRequest: base64.StdEncoding.EncodeToString(certificateSigningRequest),
	}
}

// GetClusterID returns the clusterid.
func (saIdentityRequest *ServiceAccountIdentityRequest) GetClusterID() string {
	return saIdentityRequest.ClusterID
}

// GetToken returns the token.
func (saIdentityRequest *ServiceAccountIdentityRequest) GetToken() string {
	return saIdentityRequest.Token
}

// GetPath returns the absolute path of the endpoint to contact to send a new ServiceAccountIdentityRequest.
func (saIdentityRequest *ServiceAccountIdentityRequest) GetPath() string {
	return IdentityURI
}

// GetClusterID returns the clusterid.
func (certIdentityRequest *CertificateIdentityRequest) GetClusterID() string {
	return certIdentityRequest.ClusterID
}

// GetToken returns the token.
func (certIdentityRequest *CertificateIdentityRequest) GetToken() string {
	return certIdentityRequest.DestinationClusterToken
}

// GetPath returns the absolute path of the endpoint to contact to send a new CertificateIdentityRequest.
func (certIdentityRequest *CertificateIdentityRequest) GetPath() string {
	return CertIdentityURI
}
