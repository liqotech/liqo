// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
