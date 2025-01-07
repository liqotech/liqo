// Copyright 2019-2025 The Liqo Authors
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

package eks

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
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
