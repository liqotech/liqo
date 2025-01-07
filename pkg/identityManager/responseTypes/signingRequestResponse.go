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

package responsetypes

// SigningRequestResponseType indicates the type for a signign request response.
type SigningRequestResponseType string

const (
	// SigningRequestResponseCertificate indicates that the signing request response contains a certificate
	// issued by the cluster CA.
	SigningRequestResponseCertificate SigningRequestResponseType = "Certificate"
	// SigningRequestResponseIAM indicates that the identity has been validated by the Amazon IAM service.
	SigningRequestResponseIAM SigningRequestResponseType = "IAM"
)

// AwsIdentityResponse contains the information about the created IAM user and the EKS cluster.
type AwsIdentityResponse struct {
	IamUserArn                         string
	AccessKeyID                        string
	SecretAccessKey                    string
	EksClusterName                     string
	EksClusterEndpoint                 string
	EksClusterCertificateAuthorityData []byte
	Region                             string
}

// SigningRequestResponse contains the response from an Indentity Provider.
type SigningRequestResponse struct {
	ResponseType SigningRequestResponseType

	Certificate []byte

	AwsIdentityResponse AwsIdentityResponse
}
