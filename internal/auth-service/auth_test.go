package auth_service

import (
	"github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterID/test"
	"github.com/liqotech/liqo/pkg/testUtils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"os"
	"path/filepath"
	"testing"
	"time"
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
		cluster     testUtils.Cluster
		clusterID   test.ClusterIDMock
		authService AuthServiceCtrl

		tMan tokenManagerMock

		stopChan chan struct{}
	)

	BeforeSuite(func() {

		_ = tMan.createToken()

		var err error
		cluster, _, err = testUtils.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
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

		authService = AuthServiceCtrl{
			namespace:            "default",
			clientset:            cluster.GetClient().Client(),
			saInformer:           saInformer,
			nodeInformer:         nodeInformer,
			secretInformer:       secretInformer,
			clusterId:            &clusterID,
			useTls:               false,
			credentialsValidator: &tokenValidator{},
		}
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
			credentials    auth.IdentityRequest
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
				credentials: auth.IdentityRequest{
					Token:     "",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					AllowEmptyToken: true,
				},
				expectedOutput: BeNil(),
			}),

			Entry("empty token refused", credentialValidatorTestcase{
				credentials: auth.IdentityRequest{
					Token:     "",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					AllowEmptyToken: false,
				},
				expectedOutput: HaveOccurred(),
			}),

			Entry("token accepted", credentialValidatorTestcase{
				credentials: auth.IdentityRequest{
					Token:     "token",
					ClusterID: "test1",
				},
				config: v1alpha1.AuthConfig{
					AllowEmptyToken: false,
				},
				expectedOutput: BeNil(),
			}),

			Entry("token refused", credentialValidatorTestcase{
				credentials: auth.IdentityRequest{
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

})
