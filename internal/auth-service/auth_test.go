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

package authservice

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	autherrors "github.com/liqotech/liqo/pkg/auth/errors"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	idManTest "github.com/liqotech/liqo/pkg/identityManager/testUtils"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestAuth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Suite")
}

type tokenManagerMock struct {
	token string
}

func (man *tokenManagerMock) getToken() (string, error) {
	return man.token, nil
}

func (man *tokenManagerMock) createToken() error {
	man.token = "token"
	return nil
}

var _ = Describe("Auth", func() {

	var (
		cluster         testutil.Cluster
		clusterIdentity discoveryv1alpha1.ClusterIdentity
		authService     Controller

		tMan tokenManagerMock

		stopChan chan struct{}

		csr []byte
	)

	BeforeSuite(func() {

		_ = tMan.createToken()

		var err error
		cluster, _, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		informerFactory := informers.NewSharedInformerFactoryWithOptions(cluster.GetClient(), 300*time.Second, informers.WithNamespace("default"))

		secretInformer := informerFactory.Core().V1().Secrets().Informer()
		secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

		clusterIdentity = discoveryv1alpha1.ClusterIdentity{
			ClusterID:   "default-cluster-id",
			ClusterName: "default-cluster-name",
		}

		stopChan = make(chan struct{})
		informerFactory.Start(stopChan)
		informerFactory.WaitForCacheSync(wait.NeverStop)

		namespaceManager := tenantnamespace.NewTenantNamespaceManager(cluster.GetClient())
		identityProvider := identitymanager.NewCertificateIdentityProvider(
			context.Background(), cluster.GetClient(), clusterIdentity, namespaceManager)

		config := apiserver.Config{Address: cluster.GetCfg().Host, TrustedCA: false}
		Expect(config.Complete(cluster.GetCfg(), cluster.GetClient())).To(Succeed())

		authService = Controller{
			namespace:            "default",
			clientset:            cluster.GetClient(),
			secretInformer:       secretInformer,
			localCluster:         clusterIdentity,
			namespaceManager:     namespaceManager,
			identityProvider:     identityProvider,
			credentialsValidator: &tokenValidator{},
			apiServerConfig:      config,
		}

		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		_, err = cluster.GetClient().RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		idManTest.StartTestApprover(cluster.GetClient(), stopChan)
	})

	AfterSuite(func() {
		close(stopChan)

		err := cluster.GetEnv().Stop()
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	Context("Token", func() {

		It("Create Token", func() {
			err := authService.createToken()
			Expect(err).To(BeNil())
		})

		It("Get Token", func() {
			Eventually(func() int {
				token, _ := authService.getToken()
				return len(token)
			}).Should(Equal(128))
		})

	})

	Context("Credential Validator", func() {

		type credentialValidatorTestcase struct {
			credentials    auth.ServiceAccountIdentityRequest
			authEnabled    bool
			expectedOutput types.GomegaMatcher
		}

		DescribeTable("Credential Validator table",
			func(c credentialValidatorTestcase) {
				err := authService.credentialsValidator.checkCredentials(&c.credentials, &tMan, c.authEnabled)
				Expect(err).To(c.expectedOutput)
			},

			Entry("empty token accepted", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:           "",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: "test", ClusterName: "test"},
				},
				authEnabled:    false,
				expectedOutput: BeNil(),
			}),

			Entry("empty token denied", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:           "",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: "test", ClusterName: "test"},
				},
				authEnabled:    true,
				expectedOutput: HaveOccurred(),
			}),

			Entry("token accepted", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:           "token",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: "test", ClusterName: "test"},
				},
				authEnabled:    true,
				expectedOutput: BeNil(),
			}),

			Entry("token denied", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:           "token-wrong",
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{ClusterID: "test", ClusterName: "test"},
				},
				authEnabled:    true,
				expectedOutput: HaveOccurred(),
			}),
		)

	})

	Context("Certificate Identity Creation", func() {

		var (
			oldAuthEnabled bool
		)

		type certificateTestcase struct {
			request          auth.CertificateIdentityRequest
			expectedOutput   types.GomegaMatcher
			expectedResponse func(*auth.CertificateIdentityResponse)
		}

		BeforeEach(func() {
			oldAuthEnabled = authService.authenticationEnabled
			authService.authenticationEnabled = false
		})

		AfterEach(func() {
			authService.authenticationEnabled = oldAuthEnabled
		})

		DescribeTable("Certificate Identity Creation table",
			func(c certificateTestcase) {
				req, err := testutil.FakeCSRRequest(authService.localCluster.ClusterID)
				Expect(err).To(BeNil())
				c.request.CertificateSigningRequest = base64.StdEncoding.EncodeToString(req)

				response, err := authService.handleIdentity(context.TODO(), c.request)
				Expect(err).To(c.expectedOutput)
				c.expectedResponse(response)
			},

			Entry("first creation", certificateTestcase{
				request: auth.CertificateIdentityRequest{
					ClusterIdentity:           discoveryv1alpha1.ClusterIdentity{ClusterID: "cluster1", ClusterName: "cluster1"},
					CertificateSigningRequest: string(csr),
				},
				expectedOutput: BeNil(),
				expectedResponse: func(resp *auth.CertificateIdentityResponse) {
					Expect(resp).NotTo(BeNil())
				},
			}),

			Entry("second creation", certificateTestcase{
				request: auth.CertificateIdentityRequest{
					ClusterIdentity:           discoveryv1alpha1.ClusterIdentity{ClusterID: "cluster1", ClusterName: "cluster1"},
					CertificateSigningRequest: string(csr),
				},
				expectedOutput: HaveOccurred(),
				expectedResponse: func(resp *auth.CertificateIdentityResponse) {
					Expect(resp).To(BeNil())
				},
			}),

			Entry("create different one", certificateTestcase{
				request: auth.CertificateIdentityRequest{
					ClusterIdentity:           discoveryv1alpha1.ClusterIdentity{ClusterID: "cluster2", ClusterName: "cluster2"},
					CertificateSigningRequest: string(csr),
				},
				expectedOutput: BeNil(),
				expectedResponse: func(resp *auth.CertificateIdentityResponse) {
					Expect(resp).NotTo(BeNil())
				},
			}),
		)
	})

	Context("errorHandler", func() {

		type errorHandlerTestcase struct {
			err  error
			body []byte
			code int
		}

		DescribeTable("errorHandler table",
			func(c errorHandlerTestcase) {
				recorder := httptest.NewRecorder()
				authService.handleError(recorder, c.err)

				recorder.Flush()

				body, err := io.ReadAll(recorder.Body)
				Expect(err).To(Succeed())
				Expect(string(body)).To(ContainSubstring(string(c.body)))
				Expect(recorder.Code).To(Equal(c.code))
			},

			Entry("generic error", errorHandlerTestcase{
				err:  fmt.Errorf("generic error"),
				body: []byte("generic error"),
				code: http.StatusInternalServerError,
			}),

			Entry("status error", errorHandlerTestcase{
				err:  kerrors.NewForbidden(discoveryv1alpha1.ForeignClusterGroupResource, "test", fmt.Errorf("")),
				body: []byte("forbidden"),
				code: http.StatusForbidden,
			}),

			Entry("client error", errorHandlerTestcase{
				err: &autherrors.ClientError{
					Reason: "client error",
				},
				body: []byte("client error"),
				code: http.StatusBadRequest,
			}),

			Entry("authentication error", errorHandlerTestcase{
				err: &autherrors.AuthenticationFailedError{
					Reason: "invalid token",
				},
				body: []byte("invalid token"),
				code: http.StatusUnauthorized,
			}),
		)

	})

})
