package install

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLiqoctlInstall(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ForeignClusterUtils")
}

var _ = Describe("liqoctl", func() {
	Context("install", func() {
		When("the user does not specify a tag", func() {
			It("returns a suitable tag", func() {
				tag, err := FindNewestRelease()
				Expect(err).To(BeNil())
				tag = strings.ToLower(tag)
				// Do not pick "latest", because it's not guaranteed to be a release
				Expect(tag).ToNot(Equal("latest"))
				Expect(tag).ToNot(ContainSubstring("rc"))
				Expect(tag).ToNot(ContainSubstring("alpha"))
			})
		})
	})
})