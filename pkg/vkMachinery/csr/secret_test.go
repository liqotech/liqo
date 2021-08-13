package csr

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	nodeName  = "node-name"
	namespace = "default"
)

var _ = Describe("Secret", func() {

	It("Secret Lifecycle", func() {

		client := kubernetes.NewForConfigOrDie(cluster.GetCfg())

		By("Create the CSR secret")

		Expect(createCSRSecret(ctx, client, []byte("key"), []byte("csr"), nodeName, namespace)).To(Succeed())

		secret, err := client.CoreV1().Secrets(namespace).Get(ctx, nodeName, metav1.GetOptions{})
		Expect(err).To(Succeed())
		Expect(secret).ToNot(BeNil())
		Expect(func() bool { _, ok := secret.Labels[csrSecretLabel]; return ok }()).To(BeTrue())

		keys := func(m map[string][]byte) []string {
			var res []string
			for k, v := range m {
				if len(v) > 0 {
					res = append(res, k)
				}
			}
			return res
		}

		Expect(keys(secret.Data)).To(ContainElements(csrPrivateKey, csrKey))

		By("Store the certificate in the CSR secret")

		Expect(StoreCertificate(ctx, client, []byte("cert"), namespace, nodeName)).To(Succeed())

		secret, err = client.CoreV1().Secrets(namespace).Get(ctx, nodeName, metav1.GetOptions{})
		Expect(err).To(Succeed())
		Expect(secret).ToNot(BeNil())
		Expect(keys(secret.Data)).To(ContainElements(csrPrivateKey, csrKey, csrCertificate))

		By("Get the data stored in the secret")

		key, csr, cert, err := getCSRData(ctx, client, nodeName, namespace)
		Expect(err).To(Succeed())
		Expect(key).To(Equal([]byte("key")))
		Expect(csr).To(Equal([]byte("csr")))
		Expect(cert).To(Equal([]byte("cert")))

		By("Persist the data in files")

		csrLocation := "data.csr"
		keyLocation := "data.key"
		certLocation := "data.crt"

		Expect(PersistCertificates(ctx, client, nodeName, namespace, csrLocation, keyLocation, certLocation)).To(Succeed())

		info, err := os.Stat(csrLocation)
		Expect(err).To(Succeed())
		Expect(info.Size()).To(BeNumerically(">", 0))
		info, err = os.Stat(keyLocation)
		Expect(err).To(Succeed())
		Expect(info.Size()).To(BeNumerically(">", 0))
		info, err = os.Stat(certLocation)
		Expect(err).To(Succeed())
		Expect(info.Size()).To(BeNumerically(">", 0))

		Expect(os.Remove(csrLocation)).To(Succeed())
		Expect(os.Remove(keyLocation)).To(Succeed())
		Expect(os.Remove(certLocation)).To(Succeed())

	})

})
