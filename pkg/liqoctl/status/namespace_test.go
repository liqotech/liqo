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

package status

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	nsChecker     namespaceChecker
	ctx           context.Context
	namespaceName string
	namespace     *corev1.Namespace
)

var _ = Describe("Namespace", func() {
	Describe("namespaceChecker", func() {
		BeforeEach(func() {
			ctx = context.Background()
			namespaceName = "foo"
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}
		})

		JustBeforeEach(func() {
			nsChecker = namespaceChecker{
				namespace: namespaceName,
				name:      nsCheckerName,
			}
		})

		Describe("creating a new namespaceChecker", func() {
			It("should hold the parameters passed during the creation", func() {
				nc := newNamespaceChecker(namespaceName, fake.NewSimpleClientset())
				Expect(nc.namespace).To(Equal(namespaceName))
				Expect(nc.failureReason).To(BeNil())
				Expect(nc.succeeded).To(BeFalse())
				Expect(nc.client).NotTo(BeNil())
			})
		})

		Describe("Collect() function", func() {
			When("fails to get the namespace", func() {
				It("should set succeeded field to false, and the reason of failure", func() {
					nsChecker.client = fake.NewSimpleClientset()
					Expect(nsChecker.Collect(ctx)).To(BeNil())
					Expect(nsChecker.succeeded).To(BeFalse())
					Expect(k8serror.IsNotFound(nsChecker.failureReason)).To(BeTrue())
				})
			})

			When("succeeds to get the namespace", func() {
				It("should set the succeeded field to true", func() {
					nsChecker.client = fake.NewSimpleClientset(namespace)
					Expect(nsChecker.Collect(ctx)).To(BeNil())
					Expect(nsChecker.succeeded).To(BeTrue())
					Expect(nsChecker.failureReason).To(BeNil())
				})
			})
		})

		Describe("Format() function", func() {
			When("the collection has failed", func() {
				It("should state that the check has failed", func() {
					nsChecker.succeeded = false
					nsChecker.failureReason = fmt.Errorf("unable to find namespace foo")
					msg, err := nsChecker.Format()
					Expect(err).NotTo(HaveOccurred())
					Expect(msg).To(ContainSubstring(fmt.Sprintf("%s liqo control plane namespace %s[%s]%s is not OK\n", redCross, red, nsChecker.namespace, reset)))
					Expect(msg).To(ContainSubstring(fmt.Sprintf("Reason: %s\n", nsChecker.failureReason)))
				})
			})

			When("succeeds to get the namespace", func() {
				It("should set the succeeded field to true", func() {
					nsChecker.succeeded = true
					msg, err := nsChecker.Format()
					Expect(err).NotTo(HaveOccurred())
					Expect(msg).To(ContainSubstring(fmt.Sprintf("%s liqo control plane namespace %s[%s]%s exists\n", checkMark, green, nsChecker.namespace, reset)))
				})
			})
		})

		Describe("HasSucceeded() function", func() {
			When("check succeeds", func() {
				It("should return true", func() {
					nsChecker.succeeded = true
					Expect(nsChecker.HasSucceeded()).To(BeTrue())
				})
			})

			When("check fails", func() {
				It("should return false", func() {
					nsChecker.succeeded = false
					Expect(nsChecker.HasSucceeded()).To(BeFalse())
				})
			})
		})
	})
})
