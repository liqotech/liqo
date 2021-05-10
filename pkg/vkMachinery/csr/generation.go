package csr

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"time"

	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog/v2"

	vk "github.com/liqotech/liqo/pkg/vkMachinery"
)

// GenerateVKCertificateBundle generates respectively a key and a CSR in PEM format compliant
// with the K8s kubelet-serving signer taking a name as input
func GenerateVKCertificateBundle(name string) (csrPEM []byte, keyPEM []byte, err error) {
	// Generate a new private key.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate a new private key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal the new key to DER: %v", err)
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{Type: keyutil.ECPrivateKeyBlockType, Bytes: der})

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			Organization: []string{
				"system:nodes",
			},
			CommonName: "system:node:" + name,
		},
		DNSNames: []string{"DNS:" + name},
	}
	csrPEM, err = cert.MakeCSRFromTemplate(privateKey, template)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create a csr from the private key: %v", err)
	}
	return csrPEM, keyPEM, nil
}

// WaitForApproval returns a CRT when available for a specific CSR resource. It timeouts after a while, if the CRT is not available.
func WaitForApproval(client k8s.Interface, name string) ([]byte, error) {
	ticker := time.NewTicker(3 * time.Second)
	timeout := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-timeout.C:
			return nil, fmt.Errorf("timeout elapsed waiting for the certificate %s to be forged", name)
		case <-ticker.C:
			crt, err := client.CertificatesV1beta1().CertificateSigningRequests().Get(context.TODO(), name, metav1.GetOptions{})
			if err != nil {
				klog.Errorf("Unable to get CSR: %s", err)
			}
			if len(crt.Status.Certificate) > 0 {
				klog.Infof("Certificate retrieved for CSR %s", crt.Name)
				return crt.Status.Certificate, nil
			} else {
				klog.Warningf("Certificate not available for CSR %s", crt.Name)
			}
		}
	}
}

// GenerateCSR generate a certificates/v1beta1 CSR resource for a virtual-kubelet name and PEM CSR
func GenerateVKCSR(name string, csr []byte) *certificatesv1beta1.CertificateSigningRequest {
	signerName := "kubernetes.io/kubelet-serving"
	return &certificatesv1beta1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: vk.CsrLabels,
		},
		Spec: certificatesv1beta1.CertificateSigningRequestSpec{
			Request:    csr,
			SignerName: &signerName,
			Usages: []certificatesv1beta1.KeyUsage{
				certificatesv1beta1.UsageServerAuth,
				certificatesv1beta1.UsageKeyEncipherment,
				certificatesv1beta1.UsageDigitalSignature,
			},
		},
	}
}
