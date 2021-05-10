package identityManager

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/liqotech/liqo/pkg/clusterID/test"
	"github.com/liqotech/liqo/pkg/discovery"
	idManTest "github.com/liqotech/liqo/pkg/identityManager/testUtils"
	"github.com/liqotech/liqo/pkg/tenantControlNamespace"
	"github.com/liqotech/liqo/pkg/testUtils"
)

func TestIdentityManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IdentityManager Suite")
}

var _ = Describe("IdentityManager", func() {

	var (
		cluster         testUtils.Cluster
		client          kubernetes.Interface
		localClusterID  test.ClusterIDMock
		remoteClusterID string

		namespace *v1.Namespace

		identityManager  IdentityManager
		namespaceManager tenantControlNamespace.TenantControlNamespaceManager
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

		namespaceManager = tenantControlNamespace.NewTenantControlNamespaceManager(client)
		identityManager = NewCertificateIdentityManager(cluster.GetClient().Client(), &localClusterID, namespaceManager)

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
			secret, err := identityManager.CreateIdentity(remoteClusterID)
			Expect(err).To(BeNil())
			Expect(secret).NotTo(BeNil())
			Expect(secret.Namespace).To(Equal(namespace.Name))

			Expect(secret.Labels).NotTo(BeNil())
			_, ok := secret.Labels[localIdentitySecretLabel]
			Expect(ok).To(BeTrue())
			v, ok := secret.Labels[discovery.ClusterIdLabel]
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
			csrBytes, err := identityManager.GetSigningRequest(remoteClusterID)
			Expect(err).To(BeNil())

			b, _ := pem.Decode(csrBytes)
			csr, err := x509.ParseCertificateRequest(b.Bytes)
			Expect(err).To(BeNil())
			Expect(csr.Subject.CommonName).To(Equal(localClusterID.GetClusterID()))
		})

		It("Get Signing Request with multiple secrets", func() {
			// we need that at least 1 second passed since the creation of the previous identity
			time.Sleep(1 * time.Second)

			secret, err := identityManager.CreateIdentity(remoteClusterID)
			Expect(err).To(BeNil())

			csrBytes, err := identityManager.GetSigningRequest(remoteClusterID)
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
			csrBytes, err = identityManager.GetSigningRequest(remoteClusterID)
			Expect(err).To(BeNil())

			stopChan = make(chan struct{})
			idManTest.StartTestApprover(client, stopChan)
		})

		AfterEach(func() {
			close(stopChan)
		})

		It("Approve Signing Request", func() {
			certificate, err := identityManager.ApproveSigningRequest(remoteClusterID, base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate).To(Equal([]byte("test")))
		})

		It("Retrieve Remote Certificate", func() {
			certificate, err := identityManager.GetRemoteCertificate(remoteClusterID, base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).To(BeNil())
			Expect(certificate).NotTo(BeNil())
			Expect(certificate).To(Equal([]byte("test")))
		})

		It("Retrieve Remote Certificate wrong clusterID", func() {
			certificate, err := identityManager.GetRemoteCertificate("fake", base64.StdEncoding.EncodeToString(csrBytes))
			Expect(err).NotTo(BeNil())
			Expect(kerrors.IsNotFound(err)).To(BeTrue())
			Expect(kerrors.IsBadRequest(err)).To(BeFalse())
			Expect(certificate).To(BeNil())
		})

		It("Retrieve Remote Certificate wrong CSR", func() {
			certificate, err := identityManager.GetRemoteCertificate(remoteClusterID, base64.StdEncoding.EncodeToString([]byte("fake")))
			Expect(err).NotTo(BeNil())
			Expect(kerrors.IsNotFound(err)).To(BeFalse())
			Expect(kerrors.IsBadRequest(err)).To(BeTrue())
			Expect(certificate).To(BeNil())
		})

	})

})
