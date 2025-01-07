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

package csr

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var _ = Describe("Approval functions", func() {
	When("approving a CSR", func() {
		var (
			client kubernetes.Interface

			csr certificatesv1.CertificateSigningRequest
			err error
		)

		BeforeEach(func() {
			csr = certificatesv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "csr",
					Labels: map[string]string{"liqo.io/csr": "true"},
				},
			}

			client = fake.NewSimpleClientset(&csr)
		})

		JustBeforeEach(func() {
			err = Approve(client, &csr, "LiqoApproval", "This CSR was approved by Liqo")
		})

		It("should succeed", func() { Expect(err).ToNot(HaveOccurred()) })
		It("should set the appropriate conditions", func() {
			csr, err := client.CertificatesV1().CertificateSigningRequests().Get(ctx, csr.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())

			Expect(csr.Status.Conditions).To(HaveLen(1))
			Expect(csr.Status.Conditions[0].Type).To(Equal(certificatesv1.CertificateApproved))
			Expect(csr.Status.Conditions[0].Reason).To(Equal("LiqoApproval"))
			Expect(csr.Status.Conditions[0].Message).To(Equal("This CSR was approved by Liqo"))
		})
	})
})
