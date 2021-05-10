package search_domain_operator

import (
	testUtils2 "github.com/liqotech/liqo/pkg/utils/testUtils"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/internal/discovery"
)

func TestWan(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Wan Suite")
}

var _ = Describe("Wan", func() {

	var (
		dnsServer testUtils2.DnsServer
	)

	BeforeSuite(func() {
		dnsServer = testUtils2.DnsServer{}
		dnsServer.Serve()
	})

	AfterSuite(func() {
		dnsServer.Shutdown()
	})

	Context("Wan", func() {

		It("resolve Wan", func() {
			data, err := LoadAuthDataFromDNS(dnsServer.GetAddr(), dnsServer.GetName())
			Expect(err).To(BeNil())
			Expect(data).NotTo(BeNil())
			Expect(data).To(Equal([]*discovery.AuthData{
				discovery.NewAuthData("h1.test.liqo.io.", 1234, 60),
				discovery.NewAuthData("h2.test.liqo.io.", 4321, 60),
			}))
		})

	})

})
