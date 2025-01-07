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
	"encoding/base64"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/auth"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var (
	ctx    context.Context
	cancel context.CancelFunc

	cluster       testutil.Cluster
	k8sClient     kubernetes.Interface
	localCluster  liqov1beta1.ClusterID
	remoteCluster liqov1beta1.ClusterID
	mgr           manager.Manager

	namespace *corev1.Namespace

	identityMan            IdentityManager
	identityProvider       IdentityProvider
	namespaceManager       tenantnamespace.Manager
	apiProxyURL            string
	secretIdentityResponse *auth.CertificateIdentityResponse
	certificateSecretData  map[string]string
	iamIdentityResponse    *auth.CertificateIdentityResponse
	signingIAMResponse     responsetypes.SigningRequestResponse
	iamSecretData          map[string]string
	notFoundError          error
)

func TestIdentityManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IdentityManager Suite")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
	ctx, cancel = context.WithCancel(context.Background())

	apiProxyURL = "http://192.168.0.0:8118"

	certificateSecretData = make(map[string]string)
	iamSecretData = make(map[string]string)

	localCluster = liqov1beta1.ClusterID("local-cluster-id")
	remoteCluster = liqov1beta1.ClusterID("remote-cluster-id")

	notFoundError = kerrors.NewNotFound(schema.GroupResource{
		Group:    "v1",
		Resource: "secrets",
	}, string(remoteCluster))

	var err error
	cluster, mgr, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "charts", "liqo-crds", "crds")})
	Expect(err).ToNot(HaveOccurred())

	k8sClient = cluster.GetClient()
	cnf := cluster.GetCfg()
	cl, err := client.New(cnf, client.Options{})
	Expect(err).ToNot(HaveOccurred())

	namespaceManager = tenantnamespace.NewManager(k8sClient, cl.Scheme())
	identityMan = NewCertificateIdentityManager(ctx, cl, cluster.GetClient(), cluster.GetCfg(), localCluster, namespaceManager)
	identityProvider = NewCertificateIdentityProvider(ctx, cl, cluster.GetClient(), cluster.GetCfg(), localCluster, namespaceManager)

	namespace, err = namespaceManager.CreateNamespace(ctx, remoteCluster)
	Expect(err).ToNot(HaveOccurred())

	// Make sure the namespace has been cached for subsequent retrieval.
	Eventually(func() (*corev1.Namespace, error) { return namespaceManager.GetNamespace(ctx, remoteCluster) }).Should(Equal(namespace))

	// Certificate Secret Section
	apiServerConfig := apiserver.Config{Address: "127.0.0.1", TrustedCA: false}
	Expect(apiServerConfig.Complete(cluster.GetCfg(), cl)).To(Succeed())

	signingIdentityResponse := responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseCertificate,
		Certificate:  []byte("cert"),
	}

	secretIdentityResponse, err = auth.NewCertificateIdentityResponse(
		"remoteNamespace", &signingIdentityResponse, apiServerConfig)
	Expect(err).ToNot(HaveOccurred())
	certificate, err := base64.StdEncoding.DecodeString(secretIdentityResponse.Certificate)
	Expect(err).ToNot(HaveOccurred())
	certificateSecretData[privateKeySecretKey] = "private-key-test"
	certificateSecretData[certificateSecretKey] = string(certificate)
	apiServerCa, err := base64.StdEncoding.DecodeString(secretIdentityResponse.APIServerCA)
	Expect(err).ToNot(HaveOccurred())
	certificateSecretData[apiServerCaSecretKey] = string(apiServerCa)
	certificateSecretData[APIServerURLSecretKey] = secretIdentityResponse.APIServerURL
	certificateSecretData[apiProxyURLSecretKey] = apiProxyURL
	certificateSecretData[namespaceSecretKey] = secretIdentityResponse.Namespace

	// IAM Secret Section
	signingIAMResponse = responsetypes.SigningRequestResponse{
		ResponseType: responsetypes.SigningRequestResponseIAM,
		AwsIdentityResponse: responsetypes.AwsIdentityResponse{
			IamUserArn:                         "arn:example",
			AccessKeyID:                        "key",
			SecretAccessKey:                    "secret",
			EksClusterName:                     "clustername",
			EksClusterEndpoint:                 "https://example.com",
			EksClusterCertificateAuthorityData: []byte("cert"),
			Region:                             "region",
		},
	}

	iamIdentityResponse, err = auth.NewCertificateIdentityResponse(
		"remoteNamespace", &signingIAMResponse, apiServerConfig)
	Expect(err).ToNot(HaveOccurred())
	iamSecretData[AwsAccessKeyIDSecretKey] = iamIdentityResponse.AWSIdentityInfo.AccessKeyID
	iamSecretData[AwsSecretAccessKeySecretKey] = iamIdentityResponse.AWSIdentityInfo.SecretAccessKey
	iamSecretData[AwsRegionSecretKey] = iamIdentityResponse.AWSIdentityInfo.Region
	iamSecretData[AwsEKSClusterIDSecretKey] = iamIdentityResponse.AWSIdentityInfo.EKSClusterID
	iamSecretData[AwsIAMUserArnSecretKey] = iamIdentityResponse.AWSIdentityInfo.IAMUserArn
	iamSecretData[APIServerURLSecretKey] = iamIdentityResponse.APIServerURL
	apiServerCa, err = base64.StdEncoding.DecodeString(iamIdentityResponse.APIServerCA)
	Expect(err).ToNot(HaveOccurred())
	iamSecretData[apiServerCaSecretKey] = string(apiServerCa)
	iamSecretData[apiProxyURLSecretKey] = apiProxyURL

})

var _ = AfterSuite(func() {
	cancel()
	Expect(cluster.GetEnv().Stop()).To(Succeed())
})
