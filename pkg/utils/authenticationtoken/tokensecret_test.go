package authenticationtoken

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Context("tokenSecret test", func() {

	Context("StoreInSecret", func() {

		const (
			foreignClusterID = "fc-id-1"
			liqoNamespace    = v1.NamespaceDefault

			token       = "token"
			updateToken = "token-update"
		)

		var secretName = fmt.Sprintf("%v%v", authTokenSecretNamePrefix, foreignClusterID)

		AfterEach(func() {
			Expect(clientset.CoreV1().Secrets(liqoNamespace).Delete(ctx,
				secretName,
				metav1.DeleteOptions{})).To(Succeed())
		})

		It("StoreInSecret", func() {

			By("create the secret")

			Expect(StoreInSecret(ctx, clientset, foreignClusterID, token, liqoNamespace)).To(Succeed())

			secret, err := clientset.CoreV1().Secrets(liqoNamespace).Get(ctx, secretName, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(secret).ToNot(BeNil())

			storedToken, ok := secret.Data[tokenKey]
			Expect(ok).To(BeTrue())
			Expect(string(storedToken)).To(Equal(token))

			By("update the secret")

			Expect(StoreInSecret(ctx, clientset, foreignClusterID, updateToken, liqoNamespace)).To(Succeed())

			secret, err = clientset.CoreV1().Secrets(liqoNamespace).Get(ctx, secretName, metav1.GetOptions{})
			Expect(err).To(Succeed())
			Expect(secret).ToNot(BeNil())

			storedToken, ok = secret.Data[tokenKey]
			Expect(ok).To(BeTrue())
			Expect(string(storedToken)).To(Equal(updateToken))

		})

	})

})
