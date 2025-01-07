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

package auth

import (
	"encoding/base64"
	"fmt"

	"k8s.io/klog/v2"

	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
)

// AWSIdentityInfo contains the information required by a cluster to get a valied IAM-based identity.
type AWSIdentityInfo struct {
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
	Region          string `json:"region"`
	EKSClusterID    string `json:"eksClusterID"`
	IAMUserArn      string `json:"iamUserArn"`
}

// CertificateIdentityResponse is the response on a certificate identity request.
type CertificateIdentityResponse struct {
	Namespace    string `json:"namespace"`
	Certificate  string `json:"certificate,omitempty"`
	APIServerURL string `json:"apiServerUrl"`
	APIServerCA  string `json:"apiServerCA,omitempty"`

	AWSIdentityInfo AWSIdentityInfo `json:"aws,omitempty"`
}

// HasAWSValues checks if the response has all the required AWS fields set.
func (resp *CertificateIdentityResponse) HasAWSValues() bool {
	credentials := resp.AWSIdentityInfo.AccessKeyID != "" && resp.AWSIdentityInfo.SecretAccessKey != ""
	region := resp.AWSIdentityInfo.Region != ""
	cluster := resp.AWSIdentityInfo.EKSClusterID != ""
	userArn := resp.AWSIdentityInfo.IAMUserArn != ""
	return credentials && region && cluster && userArn
}

// NewCertificateIdentityResponse makes a new CertificateIdentityResponse.
func NewCertificateIdentityResponse(
	namespace string, identityResponse *responsetypes.SigningRequestResponse,
	apiServerConfig apiserver.Config) (*CertificateIdentityResponse, error) {
	responseType := identityResponse.ResponseType

	switch responseType {
	case responsetypes.SigningRequestResponseCertificate:
		return &CertificateIdentityResponse{
			Namespace:    namespace,
			Certificate:  base64.StdEncoding.EncodeToString(identityResponse.Certificate),
			APIServerURL: apiServerConfig.Address,
			APIServerCA:  apiServerConfig.CA,
		}, nil

	case responsetypes.SigningRequestResponseIAM:
		return &CertificateIdentityResponse{
			Namespace:    namespace,
			APIServerURL: identityResponse.AwsIdentityResponse.EksClusterEndpoint,
			APIServerCA:  string(identityResponse.AwsIdentityResponse.EksClusterCertificateAuthorityData),
			AWSIdentityInfo: AWSIdentityInfo{
				EKSClusterID:    identityResponse.AwsIdentityResponse.EksClusterName,
				AccessKeyID:     identityResponse.AwsIdentityResponse.AccessKeyID,
				SecretAccessKey: identityResponse.AwsIdentityResponse.SecretAccessKey,
				Region:          identityResponse.AwsIdentityResponse.Region,
				IAMUserArn:      identityResponse.AwsIdentityResponse.IamUserArn,
			},
		}, nil

	default:
		err := fmt.Errorf("unknown response type %v", responseType)
		klog.Error(err)
		return nil, err
	}
}
