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

package secretcontroller

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	adminssionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/liqotech/liqo/pkg/consts"
)

// generateTestCertificate creates a self-signed PEM certificate with the given validity window.
func generateTestCertificate(notBefore, notAfter time.Time) []byte {
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

var _ = Describe("HandleSecret", func() {
	const (
		secretNamespace = "liqo"
		secretName      = "liqo-webhook-certs" //nolint:gosec // Just the name of the k8s secret.
		serviceName     = "liqo-webhook"
	)

	var (
		ctx    context.Context
		cl     client.Client
		scheme *runtime.Scheme
		secret *corev1.Secret
		mwhc   *adminssionregistrationv1.MutatingWebhookConfiguration
		vwhc   *adminssionregistrationv1.ValidatingWebhookConfiguration
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)
		_ = adminssionregistrationv1.AddToScheme(scheme)

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: secretNamespace,
				Annotations: map[string]string{
					consts.WebhookServiceNameAnnotationKey: serviceName,
				},
			},
		}

		mwhc = &adminssionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "liqo-mutating-webhook",
				Labels: map[string]string{
					consts.WebHookLabel: consts.WebHookLabelValue,
				},
			},
			Webhooks: []adminssionregistrationv1.MutatingWebhook{
				{
					Name: "mutating.webhook.liqo.io",
					ClientConfig: adminssionregistrationv1.WebhookClientConfig{
						CABundle: []byte("old-ca"),
					},
				},
			},
		}

		vwhc = &adminssionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "liqo-validating-webhook",
				Labels: map[string]string{
					consts.WebHookLabel: consts.WebHookLabelValue,
				},
			},
			Webhooks: []adminssionregistrationv1.ValidatingWebhook{
				{
					Name: "validating.webhook.liqo.io",
					ClientConfig: adminssionregistrationv1.WebhookClientConfig{
						CABundle: []byte("old-ca"),
					},
				},
			},
		}
	})

	Context("when certificate data is missing", func() {
		It("should generate a new certificate and patch webhooks", func() {
			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, mwhc, vwhc).Build()

			requeueIn, err := HandleSecret(ctx, cl, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(requeueIn).To(BeNumerically(">", 0))

			Expect(secret.Data).To(HaveKey("ca"))
			Expect(secret.Data).To(HaveKey("tls.crt"))
			Expect(secret.Data).To(HaveKey("tls.key"))
			Expect(secret.Data["ca"]).ToNot(BeEmpty())
			Expect(secret.Data["tls.crt"]).ToNot(BeEmpty())
			Expect(secret.Data["tls.key"]).ToNot(BeEmpty())

			updatedMWHC := &adminssionregistrationv1.MutatingWebhookConfiguration{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(mwhc), updatedMWHC)).To(Succeed())
			Expect(updatedMWHC.Webhooks[0].ClientConfig.CABundle).To(Equal(secret.Data["ca"]))

			updatedVWHC := &adminssionregistrationv1.ValidatingWebhookConfiguration{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(vwhc), updatedVWHC)).To(Succeed())
			Expect(updatedVWHC.Webhooks[0].ClientConfig.CABundle).To(Equal(secret.Data["ca"]))
		})
	})

	Context("when certificate is fresh", func() {
		It("should not regenerate and return a positive requeue duration", func() {
			bundle, err := createWebhookCertBundle(serviceName, secretNamespace)
			Expect(err).ToNot(HaveOccurred())

			secret.Data = map[string][]byte{
				"ca":      bundle.ca,
				"tls.crt": bundle.crt,
				"tls.key": bundle.key,
			}

			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, mwhc, vwhc).Build()

			requeueIn, err := HandleSecret(ctx, cl, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(requeueIn).To(BeNumerically(">", 0))

			Expect(secret.Data["tls.crt"]).To(Equal(bundle.crt))
			Expect(secret.Data["tls.key"]).To(Equal(bundle.key))
		})
	})

	Context("when certificate is past 2/3 of its lifetime", func() {
		It("should regenerate the certificate", func() {
			bundle, err := createWebhookCertBundle(serviceName, secretNamespace)
			Expect(err).ToNot(HaveOccurred())

			secret.Data = map[string][]byte{
				"ca":      bundle.ca,
				"tls.crt": bundle.crt,
				"tls.key": bundle.key,
			}

			// Replace the certificate with one that is past 2/3 of its lifetime
			// (valid from 20 minutes ago to 10 minutes from now; 2/3 point was 10 minutes ago).
			shortCrt := generateTestCertificate(time.Now().Add(-20*time.Minute), time.Now().Add(10*time.Minute))
			secret.Data["tls.crt"] = shortCrt

			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, mwhc, vwhc).Build()

			requeueIn, err := HandleSecret(ctx, cl, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(requeueIn).To(BeNumerically(">", 0))

			Expect(secret.Data["tls.crt"]).ToNot(Equal(shortCrt))
			Expect(secret.Data["tls.key"]).ToNot(Equal(bundle.key))
			Expect(secret.Data["ca"]).ToNot(Equal(bundle.ca))

			updatedMWHC := &adminssionregistrationv1.MutatingWebhookConfiguration{}
			Expect(cl.Get(ctx, client.ObjectKeyFromObject(mwhc), updatedMWHC)).To(Succeed())
			Expect(updatedMWHC.Webhooks[0].ClientConfig.CABundle).To(Equal(secret.Data["ca"]))
		})
	})

	Context("when certificate is invalid", func() {
		It("should regenerate the certificate", func() {
			secret.Data = map[string][]byte{
				"ca":      []byte("not-a-cert"),
				"tls.crt": []byte("not-a-cert"),
				"tls.key": []byte("not-a-key"),
			}

			cl = fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, mwhc, vwhc).Build()

			requeueIn, err := HandleSecret(ctx, cl, secret)
			Expect(err).ToNot(HaveOccurred())
			Expect(requeueIn).To(BeNumerically(">", 0))

			Expect(secret.Data["ca"]).ToNot(Equal([]byte("not-a-cert")))
			Expect(secret.Data["tls.crt"]).ToNot(Equal([]byte("not-a-cert")))
		})
	})
})
