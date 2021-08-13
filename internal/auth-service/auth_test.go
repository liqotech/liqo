package authservice

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
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
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	autherrors "github.com/liqotech/liqo/pkg/auth/errors"
	"github.com/liqotech/liqo/pkg/clusterid/test"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	idManTest "github.com/liqotech/liqo/pkg/identityManager/testUtils"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
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
		cluster     testutil.Cluster
		clusterID   test.ClusterIDMock
		authService Controller

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

		informerFactory := informers.NewSharedInformerFactoryWithOptions(cluster.GetClient().Client(), 300*time.Second, informers.WithNamespace("default"))

		secretInformer := informerFactory.Core().V1().Secrets().Informer()
		secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

		clusterID = test.ClusterIDMock{}
		_ = clusterID.SetupClusterID("default")

		stopChan = make(chan struct{})
		informerFactory.Start(stopChan)
		informerFactory.WaitForCacheSync(wait.NeverStop)

		namespaceManager := tenantnamespace.NewTenantNamespaceManager(cluster.GetClient().Client())
		identityProvider := identitymanager.NewCertificateIdentityProvider(
			context.Background(), cluster.GetClient().Client(), &clusterID, namespaceManager)

		authService = Controller{
			namespace:            "default",
			restConfig:           cluster.GetClient().Config(),
			clientset:            cluster.GetClient().Client(),
			secretInformer:       secretInformer,
			localClusterID:       &clusterID,
			namespaceManager:     namespaceManager,
			identityProvider:     identityProvider,
			useTLS:               false,
			credentialsValidator: &tokenValidator{},
			apiServerConfig: &v1alpha1.APIServerConfig{
				Address:   cluster.GetCfg().Host,
				TrustedCA: false,
			},
		}

		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		_, err = cluster.GetClient().Client().RbacV1().ClusterRoles().Create(context.TODO(), clusterRole, metav1.CreateOptions{})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		idManTest.StartTestApprover(cluster.GetClient().Client(), stopChan)
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
			config         v1alpha1.AuthConfig
			expectedOutput types.GomegaMatcher
		}

		DescribeTable("Credential Validator table",
			func(c credentialValidatorTestcase) {
				authService.config = &c.config
				err := authService.credentialsValidator.checkCredentials(&c.credentials, authService.getConfigProvider(), &tMan)
				Expect(err).To(c.expectedOutput)
			},

			Entry("empty token accepted", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:     "",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					EnableAuthentication: pointer.BoolPtr(false),
				},
				expectedOutput: BeNil(),
			}),

			Entry("empty token denied", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:     "",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					EnableAuthentication: pointer.BoolPtr(true),
				},
				expectedOutput: HaveOccurred(),
			}),

			Entry("token accepted", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:     "token",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					EnableAuthentication: pointer.BoolPtr(true),
				},
				expectedOutput: BeNil(),
			}),

			Entry("token denied", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:     "token-wrong",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					EnableAuthentication: pointer.BoolPtr(true),
				},
				expectedOutput: HaveOccurred(),
			}),
		)

	})

	Context("Certificate Identity Creation", func() {

		var (
			oldConfig *v1alpha1.AuthConfig
		)

		type certificateTestcase struct {
			request          auth.CertificateIdentityRequest
			expectedOutput   types.GomegaMatcher
			expectedResponse func(*auth.CertificateIdentityResponse)
		}

		BeforeEach(func() {
			oldConfig = authService.config.DeepCopy()
			authService.config.EnableAuthentication = pointer.BoolPtr(false)
		})

		AfterEach(func() {
			authService.config = oldConfig.DeepCopy()
		})

		DescribeTable("Certificate Identity Creation table",
			func(c certificateTestcase) {
				req, err := testutil.FakeCSRRequest(authService.localClusterID.GetClusterID())
				Expect(err).To(BeNil())
				c.request.CertificateSigningRequest = base64.StdEncoding.EncodeToString(req)

				response, err := authService.handleIdentity(context.TODO(), c.request)
				Expect(err).To(c.expectedOutput)
				c.expectedResponse(response)
			},

			Entry("first creation", certificateTestcase{
				request: auth.CertificateIdentityRequest{
					ClusterID:                 "cluster1",
					CertificateSigningRequest: string(csr),
				},
				expectedOutput: BeNil(),
				expectedResponse: func(resp *auth.CertificateIdentityResponse) {
					Expect(resp).NotTo(BeNil())
				},
			}),

			Entry("second creation", certificateTestcase{
				request: auth.CertificateIdentityRequest{
					ClusterID:                 "cluster1",
					CertificateSigningRequest: string(csr),
				},
				expectedOutput: HaveOccurred(),
				expectedResponse: func(resp *auth.CertificateIdentityResponse) {
					Expect(resp).To(BeNil())
				},
			}),

			Entry("create different one", certificateTestcase{
				request: auth.CertificateIdentityRequest{
					ClusterID:                 "cluster2",
					CertificateSigningRequest: string(csr),
				},
				expectedOutput: BeNil(),
				expectedResponse: func(resp *auth.CertificateIdentityResponse) {
					Expect(resp).NotTo(BeNil())
				},
			}),
		)

		It("Populate permission", func() {
			authService.config.PeeringPermission = &v1alpha1.PeeringPermission{
				Basic: []string{"test"},
			}

			err := authService.populatePermission()
			Expect(err).To(BeNil())
			Expect(len(authService.peeringPermission.Basic)).To(Equal(1))
			Expect(authService.peeringPermission.Basic[0].Name).To(Equal("test"))
		})

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

				body, err := ioutil.ReadAll(recorder.Body)
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
