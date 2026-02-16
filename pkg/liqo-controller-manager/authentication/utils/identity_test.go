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

package utils

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// generateCertificate creates a self-signed PEM certificate with the given NotBefore and NotAfter.
func generateCertificate(notBefore, notAfter time.Time) []byte {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	Expect(err).ToNot(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test", Organization: []string{"liqo.io"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	Expect(err).ToNot(HaveOccurred())

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
}

var _ = Describe("ShouldRenewCertificate", func() {
	Context("invalid input", func() {
		It("should return an error for nil input", func() {
			_, _, err := ShouldRenewCertificate(nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to decode PEM block"))
		})

		It("should return an error for non-PEM input", func() {
			_, _, err := ShouldRenewCertificate([]byte("not a pem"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to decode PEM block"))
		})

		It("should return an error for invalid certificate bytes", func() {
			invalidPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid")})
			_, _, err := ShouldRenewCertificate(invalidPEM)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse certificate"))
		})
	})

	Context("certificate within first 2/3 of lifetime", func() {
		It("should not require renewal for a fresh certificate", func() {
			// Certificate valid from now for 1 hour. We are at 0% of lifetime.
			cert := generateCertificate(time.Now(), time.Now().Add(time.Hour))

			shouldRenew, requeueIn, err := ShouldRenewCertificate(cert)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldRenew).To(BeFalse())
			// requeueIn should be approximately (2/3 * 1h) * 1.1 = ~44min
			Expect(requeueIn).To(BeNumerically(">", 30*time.Minute))
		})

		It("should not require renewal at half lifetime", func() {
			// Certificate with 1 hour lifetime, started 30 minutes ago (50% of lifetime).
			cert := generateCertificate(time.Now().Add(-30*time.Minute), time.Now().Add(30*time.Minute))

			shouldRenew, requeueIn, err := ShouldRenewCertificate(cert)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldRenew).To(BeFalse())
			// 2/3 point is at 40 min from start, we are at 30 min, so ~10 min remaining + 10%
			Expect(requeueIn).To(BeNumerically(">", 0))
		})
	})

	Context("certificate past 2/3 of lifetime", func() {
		It("should require renewal when past 2/3 lifetime", func() {
			// Certificate with 1 hour lifetime, started 50 minutes ago (83% of lifetime).
			cert := generateCertificate(time.Now().Add(-50*time.Minute), time.Now().Add(10*time.Minute))

			shouldRenew, requeueIn, err := ShouldRenewCertificate(cert)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldRenew).To(BeTrue())
			Expect(requeueIn).To(BeZero())
		})

		It("should require renewal for an expired certificate", func() {
			// Certificate expired 10 minutes ago.
			cert := generateCertificate(time.Now().Add(-time.Hour), time.Now().Add(-10*time.Minute))

			shouldRenew, requeueIn, err := ShouldRenewCertificate(cert)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldRenew).To(BeTrue())
			Expect(requeueIn).To(BeZero())
		})

		It("should require renewal at exactly the 2/3 boundary", func() {
			// Certificate with 3 hour lifetime, started 2 hours ago (exactly 2/3).
			// twoThirdsPoint = NotAfter - lifetime/3 = now + 1h - 1h = now.
			// time.Now().Before(now) is false, so shouldRenew = true.
			cert := generateCertificate(time.Now().Add(-2*time.Hour), time.Now().Add(time.Hour))

			shouldRenew, _, err := ShouldRenewCertificate(cert)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldRenew).To(BeTrue())
		})
	})

	Context("requeue duration", func() {
		It("should return a requeue duration with 10% buffer", func() {
			// Certificate valid from now for 3 hours. We are at 0%.
			// twoThirdsPoint = now + 3h - 1h = now + 2h.
			// requeueIn = 2h * 11/10 = 2h12m.
			cert := generateCertificate(time.Now(), time.Now().Add(3*time.Hour))

			shouldRenew, requeueIn, err := ShouldRenewCertificate(cert)
			Expect(err).ToNot(HaveOccurred())
			Expect(shouldRenew).To(BeFalse())
			Expect(requeueIn).To(BeNumerically("~", 2*time.Hour+12*time.Minute, 5*time.Second))
		})
	})
})
