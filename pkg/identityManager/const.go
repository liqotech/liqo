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

const defaultOrganization = "liqo.io"

const (
	localIdentitySecretLabel  = "discovery.liqo.io/local-identity"
	remoteTenantCSRLabel      = "discovery.liqo.io/remote-tenant-csr"
	certificateAvailableLabel = "discovery.liqo.io/certificate-available"
)

const (
	certificateExpireTimeAnnotation = "discovery.liqo.io/certificate-expire-time"
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

	awsAccessKeyIDSecretKey     = "awsAccessKeyID"
	awsSecretAccessKeySecretKey = "awsSecretAccessKey"
	awsRegionSecretKey          = "awsRegion"
	awsEKSClusterIDSecretKey    = "awsEksClusterID" // nolint:gosec // not a credential
	awsIAMUserArnSecretKey      = "awsIamUserArn"   // nolint:gosec // not a credential
)
