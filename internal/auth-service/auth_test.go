package authservice

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	"github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
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

// getCSR get a CertificateSigningRequest for testing purposes
func getCSR(localClusterID string) (csrBytes []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	subj := pkix.Name{
		CommonName:   localClusterID,
		Organization: []string{"Liqo"},
	}
	rawSubj := subj.ToRDNSequence()

	asn1Subj, err := asn1.Marshal(rawSubj)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	template := x509.CertificateRequest{
		RawSubject:         asn1Subj,
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	csrBytes, err = x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	csrBytes = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	})
	return csrBytes, nil
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

		saInformer := informerFactory.Core().V1().ServiceAccounts().Informer()
		saInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

		nodeInformer := informerFactory.Core().V1().Nodes().Informer()
		nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

		secretInformer := informerFactory.Core().V1().Secrets().Informer()
		secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

		clusterID = test.ClusterIDMock{}
		_ = clusterID.SetupClusterID("default")

		stopChan = make(chan struct{})
		informerFactory.Start(stopChan)
		informerFactory.WaitForCacheSync(wait.NeverStop)

		namespaceManager := tenantnamespace.NewTenantNamespaceManager(cluster.GetClient().Client())
		identityManager := identitymanager.NewCertificateIdentityManager(cluster.GetClient().Client(), &clusterID, namespaceManager)

		authService = Controller{
			namespace:            "default",
			restConfig:           cluster.GetClient().Config(),
			clientset:            cluster.GetClient().Client(),
			saInformer:           saInformer,
			nodeInformer:         nodeInformer,
			secretInformer:       secretInformer,
			localClusterID:       &clusterID,
			namespaceManager:     namespaceManager,
			identityManager:      identityManager,
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
					AllowEmptyToken: true,
				},
				expectedOutput: BeNil(),
			}),

			Entry("empty token denied", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:     "",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					AllowEmptyToken: false,
				},
				expectedOutput: HaveOccurred(),
			}),

			Entry("token accepted", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:     "token",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					AllowEmptyToken: false,
				},
				expectedOutput: BeNil(),
			}),

			Entry("token denied", credentialValidatorTestcase{
				credentials: auth.ServiceAccountIdentityRequest{
					Token:     "token-wrong",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					AllowEmptyToken: false,
				},
				expectedOutput: HaveOccurred(),
			}),
		)

	})

	Context("ServiceAccount Creation", func() {

		type serviceAccountTestcase struct {
			clusterId      string
			expectedOutput types.GomegaMatcher
		}

		DescribeTable("ServiceAccount Creation table",
			func(c serviceAccountTestcase) {
				_, err := authService.createServiceAccount(c.clusterId)
				Expect(err).To(c.expectedOutput)
			},

			Entry("first creation", serviceAccountTestcase{
				clusterId:      "cluster1",
				expectedOutput: BeNil(),
			}),

			Entry("second creation", serviceAccountTestcase{
				clusterId:      "cluster1",
				expectedOutput: HaveOccurred(),
			}),

			Entry("create different one", serviceAccountTestcase{
				clusterId:      "cluster2",
				expectedOutput: BeNil(),
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
			authService.config.AllowEmptyToken = true
		})

		AfterEach(func() {
			authService.config = oldConfig.DeepCopy()
		})

		DescribeTable("Certificate Identity Creation table",
			func(c certificateTestcase) {
				csr, err := getCSR(authService.localClusterID.GetClusterID())
				Expect(err).To(BeNil())
				c.request.CertificateSigningRequest = base64.StdEncoding.EncodeToString(csr)

				response, err := authService.handleIdentity(c.request)
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

})
