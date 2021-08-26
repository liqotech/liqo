package eks

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IAM Key Cache Test", func() {

	const (
		iamUsername        = "user-123"
		unknownIamUsername = "user-1234"
		accessKeyID        = "key-id"
		secretAccessKey    = "secret-key"
	)

	BeforeEach(func() {
		Expect(os.RemoveAll(liqoDirPath)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(liqoDirPath)).To(Succeed())
	})

	It("key lifecycle", func() {

		By("store the keys")
		Expect(storeIamAccessKey(iamUsername, accessKeyID, secretAccessKey)).To(Succeed())

		By("retrieve the keys")
		aKey, sKey, err := retrieveIamAccessKey(iamUsername)
		Expect(err).To(Succeed())
		Expect(aKey).To(Equal(accessKeyID))
		Expect(sKey).To(Equal(secretAccessKey))

		By("read unknown user")
		aKey, sKey, err = retrieveIamAccessKey(unknownIamUsername)
		Expect(err).To(Succeed())
		Expect(aKey).To(BeEmpty())
		Expect(sKey).To(BeEmpty())

	})

})
