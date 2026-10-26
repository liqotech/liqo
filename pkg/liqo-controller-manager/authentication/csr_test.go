// Copyright 2019-2026 The Liqo Authors
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

package authentication

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

var (
	oidCommonName   = asn1.ObjectIdentifier{2, 5, 4, 3}
	oidOrganization = asn1.ObjectIdentifier{2, 5, 4, 10}
	anotherOrg      = "another-org"
)

// forgeCSRWithOrganizations builds a CSR carrying an arbitrary set of Organization
// values, mimicking a malicious peer that appends extra groups to the subject.
// The RDN sequence is built explicitly so the parsed Organization order matches the
// order of the arguments (pkix.Name.ToRDNSequence would reverse it).
func forgeCSRWithOrganizations(key crypto.PrivateKey, commonName string, organizations ...string) []byte {
	rdns := make(pkix.RDNSequence, 0, len(organizations)+1)
	for _, org := range organizations {
		rdns = append(rdns, pkix.RelativeDistinguishedNameSET{
			{Type: oidOrganization, Value: org},
		})
	}
	rdns = append(rdns, pkix.RelativeDistinguishedNameSET{
		{Type: oidCommonName, Value: commonName},
	})

	asn1Subj, err := asn1.Marshal(rdns)
	Expect(err).ToNot(HaveOccurred())

	template := x509.CertificateRequest{
		RawSubject:         asn1Subj,
		SignatureAlgorithm: x509.PureEd25519,
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	Expect(err).ToNot(HaveOccurred())

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
}

// parsedOrganizations returns the Organization values as parsed from the raw CSR,
// to assert that the forged subject really carries the intended (ordered) groups.
func parsedOrganizations(csr []byte) []string {
	block, _ := pem.Decode(csr)
	Expect(block).ToNot(BeNil())
	parsed, err := x509.ParseCertificateRequest(block.Bytes)
	Expect(err).ToNot(HaveOccurred())
	return parsed.Subject.Organization
}

// forgeCSRWithSANs builds a CSR with the given (valid) subject plus a Subject
// Alternative Name, mimicking a peer trying to smuggle extra identities.
func forgeCSRWithSANs(key crypto.PrivateKey, commonName, organization string, dnsNames ...string) []byte {
	template := x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: commonName, Organization: []string{organization}},
		DNSNames:           dnsNames,
		SignatureAlgorithm: x509.PureEd25519,
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	Expect(err).ToNot(HaveOccurred())

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
}

// parsedDNSNames returns the DNS SANs parsed from the raw CSR.
func parsedDNSNames(csr []byte) []string {
	block, _ := pem.Decode(csr)
	Expect(block).ToNot(BeNil())
	parsed, err := x509.ParseCertificateRequest(block.Bytes)
	Expect(err).ToNot(HaveOccurred())
	return parsed.DNSNames
}

// tamperSignature flips the last byte of the DER-encoded CSR, corrupting the
// self-signature while keeping the ASN.1 structure parseable.
func tamperSignature(csr []byte) []byte {
	block, _ := pem.Decode(csr)
	Expect(block).ToNot(BeNil())
	der := append([]byte(nil), block.Bytes...)
	der[len(der)-1] ^= 0xff
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
}

var _ = Describe("CSR validation", func() {
	var (
		key       ed25519.PrivateKey
		publicKey []byte
	)

	BeforeEach(func() {
		_, key, _ = ed25519.GenerateKey(rand.Reader)

		var err error
		publicKey, err = x509.MarshalPKIXPublicKey(key.Public())
		Expect(err).ToNot(HaveOccurred())
	})

	Context("control plane CSR", func() {
		const clusterID = liqov1beta1.ClusterID("consumer-cluster")

		It("accepts a CSR with exactly the expected organization", func() {
			csr, err := GenerateCSRForControlPlane(key, clusterID)
			Expect(err).ToNot(HaveOccurred())
			Expect(CheckCSRForControlPlane(csr, publicKey, clusterID)).To(Succeed())
		})

		It("rejects a CSR carrying an extra organization", func() {
			csr := forgeCSRWithOrganizations(key, CommonNameControlPlaneCSR(clusterID),
				OrganizationControlPlaneCSR(), anotherOrg)
			Expect(parsedOrganizations(csr)).To(Equal([]string{OrganizationControlPlaneCSR(), anotherOrg}))
			Expect(CheckCSRForControlPlane(csr, publicKey, clusterID)).ToNot(Succeed())
		})

		It("rejects a CSR with the expected organization only at a non-zero index", func() {
			csr := forgeCSRWithOrganizations(key, CommonNameControlPlaneCSR(clusterID),
				anotherOrg, OrganizationControlPlaneCSR())
			Expect(CheckCSRForControlPlane(csr, publicKey, clusterID)).ToNot(Succeed())
		})

		It("rejects a CSR whose self-signature is invalid", func() {
			csr, err := GenerateCSRForControlPlane(key, clusterID)
			Expect(err).ToNot(HaveOccurred())
			Expect(CheckCSRForControlPlane(csr, publicKey, clusterID)).To(Succeed())
			Expect(CheckCSRForControlPlane(tamperSignature(csr), publicKey, clusterID)).ToNot(Succeed())
		})

		It("rejects a CSR carrying subject alternative names", func() {
			csr := forgeCSRWithSANs(key, CommonNameControlPlaneCSR(clusterID),
				OrganizationControlPlaneCSR(), "evil.example.com")
			Expect(parsedDNSNames(csr)).To(Equal([]string{"evil.example.com"}))
			Expect(CheckCSRForControlPlane(csr, publicKey, clusterID)).ToNot(Succeed())
		})
	})

	Context("resource slice CSR", func() {
		var (
			resourceSlice *authv1beta1.ResourceSlice
			tenantPubPEM  []byte
		)

		BeforeEach(func() {
			resourceSlice = &authv1beta1.ResourceSlice{
				ObjectMeta: metav1.ObjectMeta{Name: "slice"},
				Spec: authv1beta1.ResourceSliceSpec{
					ConsumerClusterID: ptr.To(liqov1beta1.ClusterID("consumer-cluster")),
				},
			}
			tenantPubPEM = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKey})
		})

		It("accepts a CSR with exactly the expected organization", func() {
			csr, err := GenerateCSRForResourceSlice(key, resourceSlice)
			Expect(err).ToNot(HaveOccurred())
			resourceSlice.Spec.CSR = csr
			Expect(CheckCSRForResourceSlice(tenantPubPEM, resourceSlice, true)).To(Succeed())
		})

		It("rejects a CSR carrying an extra organization (group smuggling)", func() {
			resourceSlice.Spec.CSR = forgeCSRWithOrganizations(key, CommonNameResourceSliceCSR(resourceSlice),
				OrganizationResourceSliceCSR(resourceSlice), anotherOrg)
			Expect(parsedOrganizations(resourceSlice.Spec.CSR)).To(Equal(
				[]string{OrganizationResourceSliceCSR(resourceSlice), anotherOrg}))
			Expect(CheckCSRForResourceSlice(tenantPubPEM, resourceSlice, true)).ToNot(Succeed())
		})

		It("rejects a CSR with the expected organization only at a non-zero index", func() {
			resourceSlice.Spec.CSR = forgeCSRWithOrganizations(key, CommonNameResourceSliceCSR(resourceSlice),
				anotherOrg, OrganizationResourceSliceCSR(resourceSlice))
			Expect(CheckCSRForResourceSlice(tenantPubPEM, resourceSlice, true)).ToNot(Succeed())
		})

		It("rejects a CSR whose self-signature is invalid", func() {
			csr, err := GenerateCSRForResourceSlice(key, resourceSlice)
			Expect(err).ToNot(HaveOccurred())
			resourceSlice.Spec.CSR = csr
			Expect(CheckCSRForResourceSlice(tenantPubPEM, resourceSlice, true)).To(Succeed())
			resourceSlice.Spec.CSR = tamperSignature(csr)
			Expect(CheckCSRForResourceSlice(tenantPubPEM, resourceSlice, true)).ToNot(Succeed())
		})

		It("rejects a CSR carrying subject alternative names", func() {
			resourceSlice.Spec.CSR = forgeCSRWithSANs(key, CommonNameResourceSliceCSR(resourceSlice),
				OrganizationResourceSliceCSR(resourceSlice), "evil.example.com")
			Expect(parsedDNSNames(resourceSlice.Spec.CSR)).To(Equal([]string{"evil.example.com"}))
			Expect(CheckCSRForResourceSlice(tenantPubPEM, resourceSlice, true)).ToNot(Succeed())
		})

		It("returns an error (and does not panic) when the consumer cluster ID is missing", func() {
			csr, err := GenerateCSRForResourceSlice(key, resourceSlice)
			Expect(err).ToNot(HaveOccurred())
			noID := &authv1beta1.ResourceSlice{
				ObjectMeta: metav1.ObjectMeta{Name: "slice"},
				Spec:       authv1beta1.ResourceSliceSpec{CSR: csr},
			}
			Expect(func() { _ = CheckCSRForResourceSlice(tenantPubPEM, noID, false) }).ToNot(Panic())
			Expect(CheckCSRForResourceSlice(tenantPubPEM, noID, false)).ToNot(Succeed())
		})
	})
})
