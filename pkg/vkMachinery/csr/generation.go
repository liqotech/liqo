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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"

	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"

	vk "github.com/liqotech/liqo/pkg/vkMachinery"
)

// generateVKCertificateBundle generates respectively a key and a CSR in PEM format compliant
// with the K8s kubelet-serving signer taking a name as input.
func generateVKCertificateBundle(name string, podIP net.IP) (csrPEM, keyPEM []byte, err error) {
	// Generate a new private key.
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to generate a new private key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal the new key to DER: %w", err)
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{Type: keyutil.ECPrivateKeyBlockType, Bytes: der})

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			Organization: []string{
				csrNodeGroup,
			},
			CommonName: csrNodeGroupMember + name,
		},
		DNSNames:    []string{name},
		IPAddresses: []net.IP{podIP},
	}
	csrPEM, err = cert.MakeCSRFromTemplate(privateKey, template)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create a csr from the private key: %w", err)
	}
	return csrPEM, keyPEM, nil
}

// GenerateVKCSR generate a certificates/v1 CSR resource for a virtual-kubelet name and PEM CSR.
func GenerateVKCSR(name string, csr []byte, signerName string) *certificatesv1.CertificateSigningRequest {
	return &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: vk.CsrLabels,
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:    csr,
			SignerName: signerName,
			Usages: []certificatesv1.KeyUsage{
				certificatesv1.UsageServerAuth,
				certificatesv1.UsageKeyEncipherment,
				certificatesv1.UsageDigitalSignature,
			},
		},
	}
}
