package common

import (
	"io/ioutil"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

func TestCommon(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqoctl common functions")
}

var _ = BeforeSuite(func() {
	utilruntime.Must(discoveryv1alpha1.AddToScheme(scheme.Scheme))
})

var _ = Describe("Get REST config", func() {
	When("A configuration is set", func() {
		BeforeEach(func() {
			tmpFile, err := ioutil.TempFile(os.TempDir(), "liqoctl-test-")
			Expect(err).To(BeNil())
			Expect(os.Getenv("KUBECONFIG")).To(BeEmpty())
			Expect(os.Setenv("KUBECONFIG", tmpFile.Name())).To(Succeed())
			// Dummy config from a Kind cluster
			_, err = tmpFile.Write([]byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: YQo=
    server: https://127.0.0.1:12345
  name: kind-cluster1
contexts:
- context:
    cluster: kind-cluster1
    namespace: liqo-demo
    user: kind-cluster1
  name: kind-cluster1
current-context: kind-cluster1
kind: Config
preferences: {}
users:
- name: kind-cluster1
  user:
    client-certificate-data: YQo=
    client-key-data: YQo=
`))
			Expect(err).To(BeNil())
		})
		It("Should not fail", func() {
			config, err := GetLiqoctlRestConf()
			Expect(err).To(BeNil())
			Expect(config).ToNot(BeNil())
		})
		AfterEach(func() {
			Expect(os.Remove(os.Getenv("KUBECONFIG"))).To(Succeed())
			Expect(os.Setenv("KUBECONFIG", "")).To(Succeed())
		})
	})
	When("No configuration is set", func() {
		It("Should suggest setting KUBECONFIG", func() {
			config, err := GetLiqoctlRestConf()
			Expect(config).To(BeNil())
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("KUBECONFIG"))
		})
	})
})
