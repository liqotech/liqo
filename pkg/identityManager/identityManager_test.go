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
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterid/test"
	"github.com/liqotech/liqo/pkg/discovery"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	idManTest "github.com/liqotech/liqo/pkg/identityManager/testUtils"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

type mockApiServerConfigProvider struct {
	address   string
	trustedCA bool
}

func TestIdentityManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IdentityManager Suite")
}

var _ = Describe("IdentityManager", func() {

	var (
		ctx context.Context

		cluster         testutil.Cluster
		client          kubernetes.Interface
		restConfig      *rest.Config
		localClusterID  test.ClusterIDMock
		remoteClusterID string

		namespace *v1.Namespace

		identityMan      IdentityManager
		identityProvider IdentityProvider
		namespaceManager tenantnamespace.Manager
	)

	BeforeSuite(func() {
		ctx = context.Background()

		localClusterID = test.ClusterIDMock{
			Id: "local-id",
		}
		remoteClusterID = "remote-id"

		var err error
		cluster, _, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		client = cluster.GetClient()
		restConfig = cluster.GetCfg()

		namespaceManager = tenantnamespace.NewTenantNamespaceManager(client)
		identityMan = NewCertificateIdentityManager(cluster.GetClient(), &localClusterID, namespaceManager)
		identityProvider = NewCertificateIdentityProvider(ctx, cluster.GetClient(), &localClusterID, namespaceManager)

		namespace, err = namespaceManager.CreateNamespace(remoteClusterID)
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
		// Make sure the namespace has been cached for subsequent retrieval.
		Eventually(func() (*v1.Namespace, error) { return namespaceManager.GetNamespace(remoteClusterID) }).Should(Equal(namespace))
	})

	AfterSuite(func() {
		err := cluster.GetEnv().Stop()
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	Context("Local Manager", func() {

		It("Create Identity", func() {
			secret, err := identityMan.CreateIdentity(remoteClusterID)
			Expect(err).To(BeNil())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Namespace).To(Equal(namespace.Name))

			Expect(secret.Labels).NotTo(BeNil())
			_, ok := secret.Labels[localIdentitySecretLabel]
			Expect(ok).To(BeTrue())
			v, ok := secret.Labels[discovery.ClusterIDLabel]
			Expect(ok).To(BeTrue())
			Expect(v).To(Equal(remoteClusterID))

			Expect(secret.Annotations).NotTo(BeNil())
			_, ok = secret.Annotations[certificateExpireTimeAnnotation]
			Expect(ok).To(BeTrue())

			privateKey, ok := secret.Data[privateKeySecretKey]
			Expect(ok).To(BeTrue())
			Expect(len(privateKey)).NotTo(Equal(0))

			b, _ := pem.Decode(privateKey)
			key, err := x509.ParsePKCS8PrivateKey(b.Bytes)
			Expect(key).To(BeAssignableToTypeOf(ed25519.PrivateKey{}))
			Expect(err).To(BeNil())
		})

		It("Get Signing Request", func() {
			csrBytes, err := identityMan.GetSigningRequest(remoteClusterID)
			Expect(err).To(BeNil())

			b, _ := pem.Decode(csrBytes)
			csr, err := x509.ParseCertificateRequest(b.Bytes)
			Expect(err).To(BeNil())
			Expect(csr.Subject.CommonName).To(Equal(localClusterID.GetClusterID()))
		})

		It("Get Signing Request with multiple secrets", func() {
			// we need that at least 1 second passed since the creation of the previous identity
			time.Sleep(1 * time.Second)

			secret, err := identityMan.CreateIdentity(remoteClusterID)
			Expect(err).To(BeNil())

			csrBytes, err := identityMan.GetSigningRequest(remoteClusterID)
			Expect(err).To(BeNil())

			csrBytesSecret, ok := secret.Data[csrSecretKey]
			Expect(ok).To(BeTrue())

			// check that it returns the data for the last identity
			Expect(csrBytes).To(Equal(csrBytesSecret))
		})

	})

	Context("Remote Manager", func() {

		var csrBytes []byte
		var err error
		var stopChan chan struct{}

		BeforeEach(func() {
			csrBytes, err = identityMan.GetSigningRequest(remoteClusterID)
			Expect(err).To(BeNil())

			stopChan = make(chan struct{})
			idManTest.StartTestApprover(client, stopChan)
		})

		AfterEach(func() {
			close(stopChan)
		})

		It("Approve Signing Request", func() {
			certificate, err := identityProvider.ApproveSigningRequest(remoteClusterID, base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate.Certificate).To(Equal([]byte(idManTest.FakeCRT)))
		})

		It("Retrieve Remote Certificate", func() {
			certificate, err := identityProvider.GetRemoteCertificate(remoteClusterID, namespace.Name, base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate.Certificate).To(Equal([]byte(idManTest.FakeCRT)))
		})

		It("Retrieve Remote Certificate wrong clusterid", func() {
			certificate, err := identityProvider.GetRemoteCertificate("fake", "fake", base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).NotTo(BeNil())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())
			Expect(kerrors.IsBadRequest(err)).To(BeFalse())
			Expect(certificate).To(BeNil())
		})

		It("Retrieve Remote Certificate wrong CSR", func() {
			certificate, err := identityProvider.GetRemoteCertificate(remoteClusterID, namespace.Name, base64.StdEncoding.EncodeToString([]byte("fake")))
			Expect(err).NotTo(BeNil())
			Expect(kerrors.IsNotFound(err)).To(BeFalse())
			Expect(kerrors.IsBadRequest(err)).To(BeTrue())
			Expect(certificate).To(BeNil())
		})

	})

	Context("Storage", func() {

		It("StoreCertificate", func() {
			apiServerConfig := apiserver.Config{Address: "127.0.0.1", TrustedCA: false}

			signingIdentityResponse := responsetypes.SigningRequestResponse{
				ResponseType: responsetypes.SigningRequestResponseCertificate,
				Certificate:  []byte("cert"),
			}

			identityResponse, err := auth.NewCertificateIdentityResponse(
				"remoteNamespace", &signingIdentityResponse, apiServerConfig, client, restConfig)
			Expect(err).To(BeNil())

			// store the certificate in the secret
			err = identityMan.StoreCertificate(remoteClusterID, identityResponse)
			Expect(err).To(BeNil())

			// retrieve rest config
			cnf, err := identityMan.GetConfig(remoteClusterID, "")
			Expect(err).To(BeNil())
			Expect(cnf).NotTo(BeNil())
			Expect(cnf.Host).To(Equal(
				fmt.Sprintf(
					"https://%v", apiServerConfig.Address)))

			// retrieve the remote tenant namespace
			remoteNamespace, err := identityMan.GetRemoteTenantNamespace(remoteClusterID, "")
			Expect(err).To(BeNil())
			Expect(remoteNamespace).To(Equal("remoteNamespace"))
		})

		It("StoreCertificate IAM", func() {
			apiServerConfig := apiserver.Config{Address: "127.0.0.1", TrustedCA: false}

			signingIAMResponse := responsetypes.SigningRequestResponse{
				ResponseType: responsetypes.SigningRequestResponseIAM,
				AwsIdentityResponse: responsetypes.AwsIdentityResponse{
					IamUserArn: "arn:example",
					AccessKey: &iam.AccessKey{
						AccessKeyId:     aws.String("key"),
						SecretAccessKey: aws.String("secret"),
					},
					EksCluster: &eks.Cluster{
						Name:     aws.String("clustername"),
						Endpoint: aws.String("https://example.com"),
						CertificateAuthority: &eks.Certificate{
							Data: aws.String("cert"),
						},
					},
					Region: "region",
				},
			}

			identityResponse, err := auth.NewCertificateIdentityResponse(
				"remoteNamespace", &signingIAMResponse, apiServerConfig, client, restConfig)
			Expect(err).To(BeNil())

			// store the certificate in the secret
			err = identityMan.StoreCertificate(remoteClusterID, identityResponse)
			Expect(err).To(BeNil())

			idMan, ok := identityMan.(*identityManager)
			Expect(ok).To(BeTrue())

			tokenManager := iamTokenManager{
				client:                    idMan.client,
				availableClusterIDSecrets: map[string]types.NamespacedName{},
			}
			idMan.iamTokenManager = &tokenManager

			secret, err := idMan.getSecret(remoteClusterID)
			Expect(err).To(Succeed())

			Expect(secret.Data[awsAccessKeyIDSecretKey]).To(Equal([]byte(*signingIAMResponse.AwsIdentityResponse.AccessKey.AccessKeyId)))
			Expect(secret.Data[awsSecretAccessKeySecretKey]).To(Equal([]byte(*signingIAMResponse.AwsIdentityResponse.AccessKey.SecretAccessKey)))
			Expect(secret.Data[awsRegionSecretKey]).To(Equal([]byte(signingIAMResponse.AwsIdentityResponse.Region)))
			Expect(secret.Data[awsEKSClusterIDSecretKey]).To(Equal([]byte(*signingIAMResponse.AwsIdentityResponse.EksCluster.Name)))
			Expect(secret.Data[awsIAMUserArnSecretKey]).To(Equal([]byte(identityResponse.AWSIdentityInfo.IAMUserArn)))

			// retrieve rest config
			cnf, err := identityMan.GetConfig(remoteClusterID, "")
			Expect(err).To(Succeed())
			Expect(cnf).NotTo(BeNil())
			Expect(cnf.BearerTokenFile).ToNot(BeEmpty())

			token, err := ioutil.ReadFile(cnf.BearerTokenFile)
			Expect(err).To(Succeed())
			Expect(token).ToNot(BeEmpty())

			defer os.Remove(cnf.BearerTokenFile)

			// check if the clusterID has been added in the map
			iamTokenManager, ok := idMan.iamTokenManager.(*iamTokenManager)
			Expect(ok).To(BeTrue())

			namespacedName, ok := iamTokenManager.availableClusterIDSecrets[remoteClusterID]
			Expect(ok).To(BeTrue())

			// we have to wait for at least a second to have a different token
			time.Sleep(1 * time.Second)

			err = iamTokenManager.refreshToken(ctx, remoteClusterID, namespacedName)
			Expect(err).To(Succeed())

			newToken, err := ioutil.ReadFile(cnf.BearerTokenFile)
			Expect(err).To(Succeed())
			Expect(newToken).ToNot(BeEmpty())
			Expect(newToken).ToNot(Equal(token))
		})

	})

	Context("Identity Provider", func() {

		It("Certificate Identity Provider", func() {
			idProvider := NewCertificateIdentityProvider(ctx, cluster.GetClient(), &localClusterID, namespaceManager)

			certIDManager, ok := idProvider.(*identityManager)
			Expect(ok).To(BeTrue())

			_, ok = certIDManager.IdentityProvider.(*certificateIdentityProvider)
			Expect(ok).To(BeTrue())
		})

		It("AWS IAM Identity Provider", func() {
			idProvider := NewIAMIdentityManager(cluster.GetClient(), &localClusterID, &AwsConfig{
				AwsAccessKeyID:     "KeyID",
				AwsSecretAccessKey: "Secret",
				AwsRegion:          "region",
				AwsClusterName:     "cluster-name",
			}, namespaceManager)

			certIDManager, ok := idProvider.(*identityManager)
			Expect(ok).To(BeTrue())

			_, ok = certIDManager.IdentityProvider.(*iamIdentityProvider)
			Expect(ok).To(BeTrue())
		})

	})

	Context("Identity Provider", func() {

		It("Certificate Identity Provider", func() {
			idProvider := NewCertificateIdentityProvider(ctx, cluster.GetClient(), &localClusterID, namespaceManager)

			certIDManager, ok := idProvider.(*identityManager)
			Expect(ok).To(BeTrue())

			_, ok = certIDManager.IdentityProvider.(*certificateIdentityProvider)
			Expect(ok).To(BeTrue())
		})

		It("AWS IAM Identity Provider", func() {
			idProvider := NewIAMIdentityManager(cluster.GetClient(), &localClusterID, &AwsConfig{
				AwsAccessKeyID:     "KeyID",
				AwsSecretAccessKey: "Secret",
				AwsRegion:          "region",
				AwsClusterName:     "cluster-name",
			}, namespaceManager)

			certIDManager, ok := idProvider.(*identityManager)
			Expect(ok).To(BeTrue())

			_, ok = certIDManager.IdentityProvider.(*iamIdentityProvider)
			Expect(ok).To(BeTrue())
		})

	})

})
