// Copyright 2019-2023 The Liqo Authors
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
	"encoding/base64"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	idManTest "github.com/liqotech/liqo/pkg/identityManager/testUtils"
	"github.com/liqotech/liqo/pkg/utils/csr"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("IdentityManager", func() {
	Context("Remote Manager", Ordered, func() {

		var csrBytes []byte
		var err error
		var stopChan chan struct{}

		BeforeAll(func() {
			_, csrBytes, err = csr.NewKeyAndRequest("foobar")
			Expect(err).To(BeNil())
		})

		BeforeEach(func() {
			stopChan = make(chan struct{})
			idManTest.StartTestApprover(client, stopChan)
		})

		AfterEach(func() {
			close(stopChan)
		})

		It("Approve Signing Request", func() {
			certificate, err := identityProvider.ApproveSigningRequest(remoteCluster, base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate.Certificate).To(Equal([]byte(idManTest.FakeCRT)))
		})

		It("Retrieve Remote Certificate", func() {
			certificate, err := identityProvider.GetRemoteCertificate(remoteCluster, namespace.Name, base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate.Certificate).To(Equal([]byte(idManTest.FakeCRT)))
		})

		It("Retrieve Remote Certificate wrong clusterid", func() {
			fakeIdentity := discoveryv1alpha1.ClusterIdentity{
				ClusterID:   "fake-cluster-id",
				ClusterName: "fake-cluster-name",
			}
			certificate, err := identityProvider.GetRemoteCertificate(fakeIdentity, "fake", base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).NotTo(BeNil())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())
			Expect(kerrors.IsBadRequest(err)).To(BeFalse())
			Expect(certificate).To(BeNil())
		})

		It("Retrieve Remote Certificate wrong CSR", func() {
			certificate, err := identityProvider.GetRemoteCertificate(remoteCluster, namespace.Name, base64.StdEncoding.EncodeToString([]byte("fake")))
			Expect(err).NotTo(BeNil())
			Expect(kerrors.IsNotFound(err)).To(BeFalse())
			Expect(kerrors.IsBadRequest(err)).To(BeTrue())
			Expect(certificate).To(BeNil())
		})

	})

	Context("Storage", func() {
		var key []byte

		BeforeEach(func() {
			var err error
			key, _, err = csr.NewKeyAndRequest(remoteCluster.ClusterID)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			Expect(client.CoreV1().Secrets(namespace.Name).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
		})

		commonSecretChecks := func(secret *v1.Secret) {
			Expect(secret.Namespace).To(Equal(namespace.Name))
			Expect(secret.GetLabels()).To(HaveKeyWithValue(localIdentitySecretLabel, "true"))
			Expect(secret.GetLabels()).To(HaveKeyWithValue(certificateAvailableLabel, "true"))
			Expect(secret.GetLabels()).To(HaveKeyWithValue(discovery.ClusterIDLabel, remoteCluster.ClusterID))
			Expect(secret.GetAnnotations()).To(HaveKey(certificateExpireTimeAnnotation))
			Expect(secret.Data[privateKeySecretKey]).To(Equal(key))
		}

		It("StoreCertificate", func() {
			// store the certificate in the secret
			err := identityMan.StoreIdentity(ctx, remoteCluster, namespace.Name, key, "", secretIdentityResponse)
			Expect(err).To(BeNil())

			// retrieve rest config
			cnf, err := identityMan.GetConfig(remoteCluster, "")
			Expect(err).To(BeNil())
			Expect(cnf).NotTo(BeNil())
			Expect(cnf.Host).To(Equal("https://127.0.0.1"))

			idMan, ok := identityMan.(*identityManager)
			Expect(ok).To(BeTrue())

			secret, err := idMan.getSecret(remoteCluster)
			Expect(err).To(Succeed())

			commonSecretChecks(secret)

			_, found := secret.Data[apiProxyURLSecretKey]
			Expect(found).To(BeFalse())

			// retrieve the remote tenant namespace
			remoteNamespace, err := identityMan.GetRemoteTenantNamespace(remoteCluster, "")
			Expect(err).To(BeNil())
			Expect(remoteNamespace).To(Equal("remoteNamespace"))
		})

		It("StoreCertificate IAM", func() {
			// store the certificate in the secret
			err := identityMan.StoreIdentity(ctx, remoteCluster, namespace.Name, key, apiProxyURL, iamIdentityResponse)
			Expect(err).To(BeNil())

			idMan, ok := identityMan.(*identityManager)
			Expect(ok).To(BeTrue())

			tokenManager := iamTokenManager{
				client:                    idMan.client,
				availableClusterIDSecrets: map[string]types.NamespacedName{},
				tokenFiles:                map[string]string{},
			}
			idMan.iamTokenManager = &tokenManager

			secret, err := idMan.getSecret(remoteCluster)
			Expect(err).To(Succeed())

			commonSecretChecks(secret)

			Expect(secret.Data[awsAccessKeyIDSecretKey]).To(Equal([]byte(*signingIAMResponse.AwsIdentityResponse.AccessKey.AccessKeyId)))
			Expect(secret.Data[awsSecretAccessKeySecretKey]).To(Equal([]byte(*signingIAMResponse.AwsIdentityResponse.AccessKey.SecretAccessKey)))
			Expect(secret.Data[awsRegionSecretKey]).To(Equal([]byte(signingIAMResponse.AwsIdentityResponse.Region)))
			Expect(secret.Data[awsEKSClusterIDSecretKey]).To(Equal([]byte(*signingIAMResponse.AwsIdentityResponse.EksCluster.Name)))
			Expect(secret.Data[awsIAMUserArnSecretKey]).To(Equal([]byte(iamIdentityResponse.AWSIdentityInfo.IAMUserArn)))
			Expect(secret.Data[apiProxyURLSecretKey]).To(Equal([]byte(apiProxyURL)))

			// retrieve rest config
			cnf, err := identityMan.GetConfig(remoteCluster, "")
			Expect(err).To(Succeed())
			Expect(cnf).NotTo(BeNil())
			Expect(cnf.BearerTokenFile).ToNot(BeEmpty())

			token, err := os.ReadFile(cnf.BearerTokenFile)
			Expect(err).To(Succeed())
			Expect(token).ToNot(BeEmpty())

			defer os.Remove(cnf.BearerTokenFile)

			// check if the clusterID has been added in the map
			iamTokenManager, ok := idMan.iamTokenManager.(*iamTokenManager)
			Expect(ok).To(BeTrue())

			namespacedName, ok := iamTokenManager.availableClusterIDSecrets[remoteCluster.ClusterID]
			Expect(ok).To(BeTrue())

			// we have to wait for at least a second to have a different token
			time.Sleep(1 * time.Second)

			err = iamTokenManager.refreshToken(ctx, remoteCluster, namespacedName)
			Expect(err).To(Succeed())

			newToken, err := os.ReadFile(cnf.BearerTokenFile)
			Expect(err).To(Succeed())
			Expect(newToken).ToNot(BeEmpty())
			Expect(newToken).ToNot(Equal(token))
		})

	})

	Context("Identity Provider", func() {

		It("Certificate Identity Provider", func() {
			idProvider := NewCertificateIdentityProvider(ctx, cluster.GetClient(), localCluster, namespaceManager)

			certIDManager, ok := idProvider.(*identityManager)
			Expect(ok).To(BeTrue())

			_, ok = certIDManager.IdentityProvider.(*certificateIdentityProvider)
			Expect(ok).To(BeTrue())
		})

		It("AWS IAM Identity Provider", func() {
			idProvider := NewIAMIdentityManager(cluster.GetClient(), localCluster, &AwsConfig{
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

	Context("buildConfigFromSecret", func() {

		var (
			secret *v1.Secret
		)

		JustBeforeEach(func() {
			secret = testutil.FakeSecret("test", "", certificateSecretData)
		})

		It("private key has not been set", func() {
			delete(secret.Data, privateKeySecretKey)
			config, err := buildConfigFromSecret(secret, remoteCluster)
			Expect(config).To(BeNil())
			Expect(err).To(MatchError(notFoundError))
		})

		It("cert data has not been set", func() {
			delete(secret.Data, certificateSecretKey)
			config, err := buildConfigFromSecret(secret, remoteCluster)
			Expect(config).To(BeNil())
			Expect(err).To(MatchError(notFoundError))
		})

		It("api server url has not been set", func() {
			delete(secret.Data, APIServerURLSecretKey)
			config, err := buildConfigFromSecret(secret, remoteCluster)
			Expect(config).To(BeNil())
			Expect(err).To(MatchError(notFoundError))
		})

		It("api server CA data has not been set", func() {
			delete(secret.Data, apiServerCaSecretKey)
			config, err := buildConfigFromSecret(secret, remoteCluster)
			Expect(err).To(BeNil())
			Expect(config).NotTo(BeNil())
			Expect(config.CAData).To(BeNil())
		})

		It("proxy URL has not been set", func() {
			delete(secret.Data, apiProxyURLSecretKey)
			config, err := buildConfigFromSecret(secret, remoteCluster)
			Expect(err).To(BeNil())
			Expect(config).NotTo(BeNil())
			Expect(config.Proxy).To(BeNil())
		})

		It("proxy URL invalid value", func() {
			secret.Data[apiProxyURLSecretKey] = []byte("notAn;URL\n")
			config, err := buildConfigFromSecret(secret, remoteCluster)
			Expect(err).NotTo(BeNil())
			Expect(config).To(BeNil())
		})

		It("secret contains all the needed data", func() {
			config, err := buildConfigFromSecret(secret, remoteCluster)
			Expect(err).To(BeNil())
			Expect(config).NotTo(BeNil())
			Expect(config.Proxy).NotTo(BeNil())
			Expect(config.Host).To(Equal(certificateSecretData[APIServerURLSecretKey]))
			Expect(config.TLSClientConfig.CertData).To(Equal([]byte(certificateSecretData[certificateSecretKey])))
			Expect(config.TLSClientConfig.CAData).To(Equal([]byte(certificateSecretData[apiServerCaSecretKey])))
			Expect(config.TLSClientConfig.KeyData).To(Equal([]byte(certificateSecretData[privateKeySecretKey])))
		})

	})

	Context("iamTokenManager.getConfig", func() {

		var (
			secret       *v1.Secret
			tokenManager iamTokenManager
		)
		BeforeEach(func() {
			idMan, ok := identityMan.(*identityManager)
			Expect(ok).To(BeTrue())

			tokenManager = iamTokenManager{
				client:                    idMan.client,
				availableClusterIDSecrets: map[string]types.NamespacedName{},
				tokenFiles:                map[string]string{},
			}

		})
		JustBeforeEach(func() {
			secret = testutil.FakeSecret("test", "", iamSecretData)
		})

		It("api server url has not been set", func() {
			delete(secret.Data, APIServerURLSecretKey)
			config, err := tokenManager.getConfig(secret, remoteCluster)
			Expect(config).To(BeNil())
			Expect(err).To(MatchError(notFoundError))
		})

		It("api server CA data has not been set", func() {
			delete(secret.Data, apiServerCaSecretKey)
			config, err := tokenManager.getConfig(secret, remoteCluster)
			Expect(config).To(BeNil())
			Expect(err).To(MatchError(notFoundError))
		})

		It("proxy URL has not been set", func() {
			delete(secret.Data, apiProxyURLSecretKey)
			config, err := tokenManager.getConfig(secret, remoteCluster)
			Expect(err).To(BeNil())
			Expect(config).NotTo(BeNil())
			Expect(config.Proxy).To(BeNil())
		})

		It("proxy URL invalid value", func() {
			secret.Data[apiProxyURLSecretKey] = []byte("notAn;URL\n")
			config, err := tokenManager.getConfig(secret, remoteCluster)
			Expect(err).NotTo(BeNil())
			Expect(config).To(BeNil())
		})

		It("aws region data has not been set", func() {
			delete(secret.Data, awsRegionSecretKey)
			config, err := tokenManager.getConfig(secret, remoteCluster)
			Expect(config).To(BeNil())
			Expect(err).To(MatchError(notFoundError))
		})

		It("aws access ID data has not been set", func() {
			delete(secret.Data, awsAccessKeyIDSecretKey)
			config, err := tokenManager.getConfig(secret, remoteCluster)
			Expect(config).To(BeNil())
			Expect(err).To(MatchError(notFoundError))
		})

		It("aws secret access ID data has not been set", func() {
			delete(secret.Data, awsSecretAccessKeySecretKey)
			config, err := tokenManager.getConfig(secret, remoteCluster)
			Expect(config).To(BeNil())
			Expect(err).To(MatchError(notFoundError))
		})

		It("aws eks cluster ID data has not been set", func() {
			delete(secret.Data, awsEKSClusterIDSecretKey)
			config, err := tokenManager.getConfig(secret, remoteCluster)
			Expect(config).To(BeNil())
			Expect(err).To(MatchError(notFoundError))
		})

		It("secret contains all the needed data", func() {
			config, err := tokenManager.getConfig(secret, remoteCluster)
			Expect(err).To(BeNil())
			Expect(config).NotTo(BeNil())
			Expect(config.Proxy).NotTo(BeNil())
			Expect(config.Host).To(Equal(iamSecretData[APIServerURLSecretKey]))
			Expect(config.TLSClientConfig.CAData).To(Equal([]byte(iamSecretData[apiServerCaSecretKey])))
		})

	})

})
