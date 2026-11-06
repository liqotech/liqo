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

package tenantcontroller

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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

const (
	// testCertificateTTL is the lifetime requested through the CSR expirationSeconds field.
	testCertificateTTL = 10 * time.Minute
	// testCertificateBackdate is the amount by which the Kubernetes CSR signer backdates the certificate NotBefore.
	testCertificateBackdate = 5 * time.Minute

	testClusterID = liqov1beta1.ClusterID("consumer")
	testNamespace = "liqo-tenant-consumer"
)

// expectedRequeue returns the delay until the next renewal check for a certificate issued now with the given TTL.
// The x509 lifetime is longer than the requested one, as the signer backdates the NotBefore by 5 minutes, and the
// check is scheduled at 2/3 of it plus the 10% buffer applied by certificate.NextRenewalCheck.
func expectedRequeue(ttl time.Duration) time.Duration {
	return 11 * (ttl - (ttl+testCertificateBackdate)/3) / 10
}

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
	issued := time.Now().Add(-expectedRequeue(testCertificateTTL))
	return newTestCertificate(issued.Add(-testCertificateBackdate), issued.Add(testCertificateTTL))
}

// stubIdentityProvider is an identity manager stub returning a preconfigured certificate.
type stubIdentityProvider struct {
	cert    []byte
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
	return &authv1beta1.AuthParams{SignedCRT: s.cert}, nil
}

// stubNamespaceManager is a tenant namespace manager stub, avoiding the interaction with the API server.
type stubNamespaceManager struct{}

func (stubNamespaceManager) CreateNamespace(context.Context, liqov1beta1.ClusterID) (*corev1.Namespace, error) {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}, nil
}

func (stubNamespaceManager) ForgeNamespace(liqov1beta1.ClusterID, *string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
}

func (stubNamespaceManager) GetNamespace(context.Context, liqov1beta1.ClusterID) (*corev1.Namespace, error) {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}, nil
}

func (stubNamespaceManager) BindClusterRoles(context.Context, liqov1beta1.ClusterID,
	metav1.Object, ...*rbacv1.ClusterRole) ([]*rbacv1.RoleBinding, error) {
	return nil, nil
}

func (stubNamespaceManager) UnbindClusterRoles(context.Context, liqov1beta1.ClusterID, ...*rbacv1.ClusterRole) error {
	return nil
}

func (stubNamespaceManager) BindClusterRolesClusterWide(context.Context, liqov1beta1.ClusterID,
	metav1.Object, ...*rbacv1.ClusterRole) ([]*rbacv1.ClusterRoleBinding, error) {
	return nil, nil
}

func (stubNamespaceManager) UnbindClusterRolesClusterWide(context.Context, liqov1beta1.ClusterID, ...*rbacv1.ClusterRole) error {
	return nil
}

var _ tenantnamespace.Manager = stubNamespaceManager{}

// newTestTenant returns a Tenant whose handshake can be verified, together with the nonce Secret it refers to.
func newTestTenant(currentCert []byte) (*authv1beta1.Tenant, *corev1.Secret) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	Expect(err).ToNot(HaveOccurred())

	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	Expect(err).ToNot(HaveOccurred())

	csrBytes, err := authentication.GenerateCSRForControlPlane(priv, testClusterID)
	Expect(err).ToNot(HaveOccurred())

	nonce := make([]byte, 32)
	_, err = rand.Read(nonce)
	Expect(err).ToNot(HaveOccurred())

	tenant := &authv1beta1.Tenant{
		ObjectMeta: metav1.ObjectMeta{
			Name:       string(testClusterID),
			Namespace:  testNamespace,
			Finalizers: []string{tenantControllerFinalizer},
		},
		Spec: authv1beta1.TenantSpec{
			ClusterID: testClusterID,
			PublicKey: pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
			CSR:       csrBytes,
			Signature: ed25519.Sign(priv, nonce),
		},
	}
	if currentCert != nil {
		tenant.Status.AuthParams = &authv1beta1.AuthParams{SignedCRT: currentCert}
	}

	nonceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "liqo-nonce",
			Namespace: testNamespace,
			Labels: map[string]string{
				consts.RemoteClusterID:     string(testClusterID),
				consts.NonceSecretLabelKey: "true",
			},
		},
		Data: map[string][]byte{consts.NonceSecretField: nonce},
	}

	return tenant, nonceSecret
}

var _ = Describe("Tenant certificate renewal", func() {
	var (
		reconciler *TenantReconciler
		provider   *stubIdentityProvider
		fakeClient client.Client
		request    ctrl.Request
	)

	BeforeEach(func() {
		provider = &stubIdentityProvider{}
		request = ctrl.Request{NamespacedName: types.NamespacedName{
			Name: string(testClusterID), Namespace: testNamespace,
		}}
	})

	JustBeforeEach(func() {
		reconciler = &TenantReconciler{
			Client:           fakeClient,
			Scheme:           newTestScheme(),
			EventRecorder:    record.NewFakeRecorder(100),
			IdentityProvider: provider,
			NamespaceManager: stubNamespaceManager{},
			// Preconfigure the ClusterRoles, so that the reconciler does not have to retrieve them.
			tenantClusterRoles:            []*rbacv1.ClusterRole{{ObjectMeta: metav1.ObjectMeta{Name: "remote-controlplane"}}},
			tenantClusterRolesClusterWide: []*rbacv1.ClusterRole{{ObjectMeta: metav1.ObjectMeta{Name: "virtual-kubelet-remote-clusterwide"}}},
		}
	})

	When("the certificate is issued for the first time", func() {
		BeforeEach(func() {
			tenant, nonceSecret := newTestTenant(nil)
			provider.cert = newFreshTestCertificate()
			fakeClient = fake.NewClientBuilder().WithScheme(newTestScheme()).
				WithStatusSubresource(&authv1beta1.Tenant{}).WithObjects(tenant, nonceSecret).Build()
		})

		It("should schedule the next renewal check", func() {
			result, err := reconciler.Reconcile(context.Background(), request)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider.options.IsRenew).To(BeFalse())
			Expect(result.RequeueAfter).To(BeNumerically("~", expectedRequeue(testCertificateTTL), 5*time.Second))
		})
	})

	When("the certificate is past the 2/3 of its lifetime", func() {
		var currentCert []byte

		BeforeEach(func() {
			currentCert = newDueTestCertificate()
			tenant, nonceSecret := newTestTenant(currentCert)
			provider.cert = newFreshTestCertificate()
			fakeClient = fake.NewClientBuilder().WithScheme(newTestScheme()).
				WithStatusSubresource(&authv1beta1.Tenant{}).WithObjects(tenant, nonceSecret).Build()
		})

		It("should renew it and schedule the next renewal check", func() {
			result, err := reconciler.Reconcile(context.Background(), request)
			Expect(err).ToNot(HaveOccurred())
			Expect(provider.options.IsRenew).To(BeTrue())
			Expect(result.RequeueAfter).To(BeNumerically("~", expectedRequeue(testCertificateTTL), 5*time.Second))

			// The renewed certificate is stored in the Tenant status.
			var tenant authv1beta1.Tenant
			Expect(fakeClient.Get(context.Background(), request.NamespacedName, &tenant)).To(Succeed())
			Expect(tenant.Status.AuthParams).ToNot(BeNil())
			Expect(tenant.Status.AuthParams.SignedCRT).To(Equal(provider.cert))
			Expect(tenant.Status.AuthParams.SignedCRT).ToNot(Equal(currentCert))
		})
	})
})

// newTestScheme returns a scheme with the types used by the Tenant reconciler.
func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(rbacv1.AddToScheme(scheme)).To(Succeed())
	Expect(authv1beta1.AddToScheme(scheme)).To(Succeed())
	return scheme
}
