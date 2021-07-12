package identitymanager

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterid/test"
	"github.com/liqotech/liqo/pkg/discovery"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	idManTest "github.com/liqotech/liqo/pkg/identityManager/testUtils"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
	testUtils "github.com/liqotech/liqo/pkg/utils/testUtils"
)

type mockApiServerConfigProvider struct {
	address   string
	port      string
	trustedCA bool
}

func newMockApiServerConfigProvider(address, port string, trustedCA bool) utils.ApiServerConfigProvider {
	return &mockApiServerConfigProvider{
		address:   address,
		port:      port,
		trustedCA: trustedCA,
	}
}

func (mock *mockApiServerConfigProvider) GetAPIServerConfig() *configv1alpha1.APIServerConfig {
	return &configv1alpha1.APIServerConfig{
		Address:   mock.address,
		Port:      mock.port,
		TrustedCA: mock.trustedCA,
	}
}

func TestIdentityManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IdentityManager Suite")
}

var _ = Describe("IdentityManager", func() {

	var (
		cluster         testUtils.Cluster
		client          kubernetes.Interface
		restConfig      *rest.Config
		localClusterID  test.ClusterIDMock
		remoteClusterID string

		namespace *v1.Namespace

		identityMan      IdentityManager
		namespaceManager tenantnamespace.Manager
	)

	BeforeSuite(func() {
		localClusterID = test.ClusterIDMock{
			Id: "localID",
		}
		remoteClusterID = "remoteID"

		var err error
		cluster, _, err = testUtils.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		client = cluster.GetClient().Client()
		restConfig = cluster.GetCfg()

		namespaceManager = tenantnamespace.NewTenantNamespaceManager(client)
		identityMan = NewCertificateIdentityManager(cluster.GetClient().Client(), &localClusterID, namespaceManager)

		namespace, err = namespaceManager.CreateNamespace(remoteClusterID)
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
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
			_, err = x509.ParsePKCS1PrivateKey(b.Bytes)
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
			certificate, err := identityMan.ApproveSigningRequest(remoteClusterID, base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate.Certificate).To(Equal([]byte(idManTest.FakeCRT)))
		})

		It("Retrieve Remote Certificate", func() {
			certificate, err := identityMan.GetRemoteCertificate(remoteClusterID, base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate.Certificate).To(Equal([]byte(idManTest.FakeCRT)))
		})

		It("Retrieve Remote Certificate wrong clusterid", func() {
			certificate, err := identityMan.GetRemoteCertificate("fake", base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).NotTo(BeNil())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())
			Expect(kerrors.IsBadRequest(err)).To(BeFalse())
			Expect(certificate.Certificate).To(BeNil())
		})

		It("Retrieve Remote Certificate wrong CSR", func() {
			certificate, err := identityMan.GetRemoteCertificate(remoteClusterID, base64.StdEncoding.EncodeToString([]byte("fake")))
			Expect(err).NotTo(BeNil())
			Expect(kerrors.IsNotFound(err)).To(BeFalse())
			Expect(kerrors.IsBadRequest(err)).To(BeTrue())
			Expect(certificate.Certificate).To(BeNil())
		})

	})

	Context("Storage", func() {

		It("StoreCertificate", func() {
			apiServerConfig := newMockApiServerConfigProvider("127.0.0.1", "6443", false)

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
					"https://%v:%v", apiServerConfig.GetAPIServerConfig().Address,
					apiServerConfig.GetAPIServerConfig().Port)))

			// retrieve the remote tenant namespace
			remoteNamespace, err := identityMan.GetRemoteTenantNamespace(remoteClusterID, "")
			Expect(err).To(BeNil())
			Expect(remoteNamespace).To(Equal("remoteNamespace"))
		})

	})

	Context("Identity Provider", func() {

		It("Certificate Identity Provider", func() {
			idProvider := NewCertificateIdentityManager(cluster.GetClient().Client(), &localClusterID, namespaceManager)

			certIDManager, ok := idProvider.(*identityManager)
			Expect(ok).To(BeTrue())

			_, ok = certIDManager.identityProvider.(*certificateIdentityProvider)
			Expect(ok).To(BeTrue())
		})

		It("AWS IAM Identity Provider", func() {
			idProvider := NewIAMIdentityManager(cluster.GetClient().Client(), &localClusterID, &AwsConfig{
				AwsAccessKeyID:     "KeyID",
				AwsSecretAccessKey: "Secret",
				AwsRegion:          "region",
				AwsClusterName:     "cluster-name",
			}, namespaceManager)

			certIDManager, ok := idProvider.(*identityManager)
			Expect(ok).To(BeTrue())

			_, ok = certIDManager.identityProvider.(*iamIdentityProvider)
			Expect(ok).To(BeTrue())
		})

	})

})
