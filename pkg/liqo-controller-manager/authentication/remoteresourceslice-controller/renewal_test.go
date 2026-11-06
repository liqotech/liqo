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

package remoteresourceslicecontroller

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/utils/certificate"
)

const (
	// testCertificateTTL is the lifetime requested through the CSR expirationSeconds field.
	testCertificateTTL = 10 * time.Minute
	// testCertificateBackdate is the amount by which the Kubernetes CSR signer backdates the certificate NotBefore.
	testCertificateBackdate = 5 * time.Minute
)

var defaultExpectedRequeue = 11 * (testCertificateTTL - (testCertificateTTL+testCertificateBackdate)/3) / 10

// newTestCertificate returns a self-signed PEM certificate with the given validity period.
func newTestCertificate(notBefore, notAfter time.Time) []byte {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	Expect(err).ToNot(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	Expect(err).ToNot(HaveOccurred())

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// newFreshTestCertificate returns a certificate mimicking the one issued by the Kubernetes CSR signer now.
func newFreshTestCertificate() []byte {
	return newTestCertificate(time.Now().Add(-testCertificateBackdate), time.Now().Add(testCertificateTTL))
}

// newDueTestCertificate returns a certificate issued at the previous renewal check, i.e. past the 2/3 of its lifetime.
func newDueTestCertificate() []byte {
	issued := time.Now().Add(-defaultExpectedRequeue)
	return newTestCertificate(issued.Add(-testCertificateBackdate), issued.Add(testCertificateTTL))
}

// stubIdentityProvider is an identity manager stub returning a preconfigured certificate.
type stubIdentityProvider struct {
	cert    []byte
	err     error
	options *identitymanager.SigningRequestOptions
}

func (s *stubIdentityProvider) GetRemoteCertificate(context.Context, *identitymanager.SigningRequestOptions) (
	*responsetypes.SigningRequestResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubIdentityProvider) ApproveSigningRequest(context.Context, *identitymanager.SigningRequestOptions) (
	*responsetypes.SigningRequestResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubIdentityProvider) ForgeAuthParams(_ context.Context, options *identitymanager.SigningRequestOptions) (
	*authv1beta1.AuthParams, error) {
	s.options = options
	if s.err != nil {
		return nil, s.err
	}
	return &authv1beta1.AuthParams{SignedCRT: s.cert}, nil
}

// newTestObjects returns a ResourceSlice and the associated Tenant, sharing the same key pair.
func newTestObjects(currentCert []byte) (*authv1beta1.ResourceSlice, *authv1beta1.Tenant) {
	const (
		clusterID = liqov1beta1.ClusterID("consumer")
		namespace = "liqo-tenant-consumer"
	)

	resourceSlice := &authv1beta1.ResourceSlice{
		ObjectMeta: metav1.ObjectMeta{Name: "slice", Namespace: namespace},
		Spec:       authv1beta1.ResourceSliceSpec{ConsumerClusterID: ptrTo(clusterID)},
	}
	if currentCert != nil {
		resourceSlice.Status.AuthParams = &authv1beta1.AuthParams{SignedCRT: currentCert}
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	Expect(err).ToNot(HaveOccurred())

	resourceSlice.Spec.CSR, err = authentication.GenerateCSRForResourceSlice(priv, resourceSlice)
	Expect(err).ToNot(HaveOccurred())

	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	Expect(err).ToNot(HaveOccurred())

	tenant := &authv1beta1.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: "consumer", Namespace: namespace},
		Spec: authv1beta1.TenantSpec{
			ClusterID: clusterID,
			PublicKey: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
		},
	}

	return resourceSlice, tenant
}

func ptrTo[T any](value T) *T {
	return &value
}

// renewalTestCase describes a ResourceSlice certificate signing scenario.
type renewalTestCase struct {
	// currentCert returns the certificate already stored in the ResourceSlice status, nil if not present.
	currentCert func() []byte
	// issuedCert returns the certificate issued by the identity provider, nil to return the current one (cache hit).
	issuedCert func() []byte
	// expectRenew is whether the identity provider is expected to be asked for a renewal.
	expectRenew bool
	// expectedRequeue is the delay expected for the next renewal check.
	expectedRequeue time.Duration
}

var _ = Describe("ResourceSlice certificate renewal", func() {
	var (
		reconciler *RemoteResourceSliceReconciler
		provider   *stubIdentityProvider
	)

	BeforeEach(func() {
		provider = &stubIdentityProvider{}
		reconciler = &RemoteResourceSliceReconciler{
			eventRecorder:    record.NewFakeRecorder(100),
			identityProvider: provider,
		}
	})

	DescribeTable("should always schedule the next renewal check",
		func(c renewalTestCase) {
			var currentCert []byte
			if c.currentCert != nil {
				currentCert = c.currentCert()
			}
			resourceSlice, tenant := newTestObjects(currentCert)

			provider.cert = currentCert
			if c.issuedCert != nil {
				provider.cert = c.issuedCert()
			}

			requeueIn, err := reconciler.handleAuthenticationStatus(context.Background(), resourceSlice, tenant)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider.options.IsRenew).To(Equal(c.expectRenew))
			Expect(resourceSlice.Status.AuthParams.SignedCRT).To(Equal(provider.cert))
			Expect(requeueIn).To(BeNumerically("~", c.expectedRequeue, 5*time.Second))
		},
		Entry("when the certificate is issued for the first time", renewalTestCase{
			issuedCert:      newFreshTestCertificate,
			expectRenew:     false,
			expectedRequeue: defaultExpectedRequeue,
		}),
		Entry("when the certificate is not yet due for renewal", renewalTestCase{
			currentCert:     newFreshTestCertificate,
			expectRenew:     false,
			expectedRequeue: defaultExpectedRequeue,
		}),
		Entry("when the certificate is past 2/3 of its lifetime", renewalTestCase{
			currentCert:     newDueTestCertificate,
			issuedCert:      newFreshTestCertificate,
			expectRenew:     true,
			expectedRequeue: defaultExpectedRequeue,
		}),
		Entry("when the issued certificate is already due for renewal", renewalTestCase{
			issuedCert: func() []byte {
				return newTestCertificate(time.Now().Add(-time.Hour), time.Now().Add(-10*time.Minute))
			},
			expectRenew:     false,
			expectedRequeue: certificate.RenewalRetryInterval,
		}),
	)

	When("the identity provider fails", func() {
		It("should return the error", func() {
			resourceSlice, tenant := newTestObjects(nil)
			provider.err = errors.New("signing failed")

			_, err := reconciler.handleAuthenticationStatus(context.Background(), resourceSlice, tenant)
			Expect(err).To(HaveOccurred())
		})
	})
})
