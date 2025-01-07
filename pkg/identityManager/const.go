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

import "github.com/liqotech/liqo/pkg/consts"

const (
	localIdentitySecretLabel = "liqo.io/local-identity" //nolint:gosec // not a credential
	remoteTenantCSRLabel     = "liqo.io/remote-tenant-csr"
	// CertificateAvailableLabel is the label used to identify the secrets containing a certificate.
	CertificateAvailableLabel = "liqo.io/certificate-available"
)

const (
	localClusterIDTagKey  = "liqo.io/local-cluster-id"
	remoteClusterIDTagKey = "liqo.io/remote-cluster-id"
	managedByTagKey       = "liqo.io/managed-by"
	managedByTagValue     = "liqo"
	identityTypeTagKey    = consts.IdentityTypeLabelKey
)

const (
	certificateExpireTimeAnnotation = "liqo.io/certificate-expire-time"
)

const (
	identitySecretRoot      = "liqo-identity"
	remoteCertificateSecret = "liqo-remote-certificate"

	privateKeySecretKey  = "private-key"
	csrSecretKey         = "csr"
	certificateSecretKey = "certificate"
	// APIServerURLSecretKey key used for the api server url inside the secret.
	APIServerURLSecretKey = "apiServerUrl"
	apiProxyURLSecretKey  = "proxyURL"
	apiServerCaSecretKey  = "apiServerCa"
	namespaceSecretKey    = "namespace"

	// AwsAccessKeyIDSecretKey is the key used for the AWS access key ID inside the secret.
	AwsAccessKeyIDSecretKey = "awsAccessKeyID"
	// AwsSecretAccessKeySecretKey is the key used for the AWS secret access key inside the secret.
	AwsSecretAccessKeySecretKey = "awsSecretAccessKey"
	// AwsRegionSecretKey is the key used for the AWS region inside the secret.
	AwsRegionSecretKey = "awsRegion"
	// AwsEKSClusterIDSecretKey is the key used for the AWS EKS cluster ID inside the secret.
	AwsEKSClusterIDSecretKey = "awsEksClusterID" //nolint:gosec // not a credential
	// AwsIAMUserArnSecretKey is the key used for the AWS IAM user ARN inside the secret.
	AwsIAMUserArnSecretKey = "awsIamUserArn" //nolint:gosec // not a credential
)
