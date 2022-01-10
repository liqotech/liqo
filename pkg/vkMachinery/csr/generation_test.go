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

package csr

import (
	"crypto/x509"
	"encoding/pem"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Assert the correctness of the virtual-node CSR", func() {
	var (
		podNameT string
		podIP    net.IP
		csrPEM   []byte
		keyPEM   []byte
		csr      *x509.CertificateRequest
		csrBlock *pem.Block
	)
	When("Requesting a certificate for a virtual node", func() {
		BeforeEach(func() {
			podNameT = "podName"
			podIP = net.ParseIP("10.0.0.1").To4()
		})

		JustBeforeEach(func() {
			csrPEM, keyPEM, err = generateVKCertificateBundle(podNameT, podIP)
		})

		It("Should not trigger any error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(csrPEM).NotTo(BeNil())
			Expect(keyPEM).NotTo(BeNil())
		})

		Context("When decoding the certificate", func() {

			BeforeEach(func() {
				csrBlock, _ = pem.Decode(csrPEM)
				csr, err = x509.ParseCertificateRequest(csrBlock.Bytes)
			})

			It("Should be a valid x509 certificate", func() {
				Expect(csrBlock).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})

			It("Should include the right subject alternative names (SAN)", func() {
				Expect(csr.IPAddresses).To(HaveLen(1))
				Expect(csr.DNSNames).To(HaveLen(1))
				Expect(csr.IPAddresses[0]).To(BeEquivalentTo(podIP))
				Expect(csr.DNSNames[0]).To(BeEquivalentTo(podNameT))
			})

			It("Should include the right Common Name (CN)", func() {
				Expect(csr.Subject.CommonName).To(BeEquivalentTo(csrNodeGroupMember + podNameT))
			})

			It("Should include the right list of Organizations (O)", func() {
				Expect(csr.Subject.Organization).To(HaveLen(1))
				Expect(csr.Subject.Organization[0]).To(BeEquivalentTo(csrNodeGroup))
			})
		})

	})
})
