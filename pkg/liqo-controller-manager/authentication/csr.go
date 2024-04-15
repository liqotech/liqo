// Copyright 2019-2024 The Liqo Authors
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
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// CSRChecker is a function that checks a CSR.
type CSRChecker func(*x509.CertificateRequest) error

// GenerateCSRForResourceSlice generates a new CSR given a private key and a resource slice.
func GenerateCSRForResourceSlice(key ed25519.PrivateKey,
	resourceSlice *authv1alpha1.ResourceSlice) (csrBytes []byte, err error) {
	return generateCSR(key, CommonNameResourceSliceCSR(resourceSlice), OrganizationResourceSliceCSR(resourceSlice))
}

// CommonNameResourceSliceCSR returns the common name for a resource slice CSR.
func CommonNameResourceSliceCSR(resourceSlice *authv1alpha1.ResourceSlice) string {
	return fmt.Sprintf("%s-%s", resourceSlice.Spec.ConsumerClusterIdentity.ClusterID, resourceSlice.Name)
}

// OrganizationResourceSliceCSR returns the organization for a resource slice CSR.
func OrganizationResourceSliceCSR(resourceSlice *authv1alpha1.ResourceSlice) string {
	return resourceSlice.Spec.ConsumerClusterIdentity.ClusterID
}

// GenerateCSRForControlPlane generates a new CSR given a private key and a subject.
func GenerateCSRForControlPlane(key ed25519.PrivateKey, clusterID string) (csrBytes []byte, err error) {
	return generateCSR(key, CommonNameControlPlaneCSR(clusterID), OrganizationControlPlaneCSR())
}

// CommonNameControlPlaneCSR returns the common name for a control plane CSR.
func CommonNameControlPlaneCSR(clusterID string) string {
	return clusterID
}

// OrganizationControlPlaneCSR returns the organization for a control plane CSR.
func OrganizationControlPlaneCSR() string {
	return "liqo.io"
}

// CheckCSRForControlPlane checks a CSR for a control plane.
func CheckCSRForControlPlane(csr, publicKey []byte, remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity) error {
	return checkCSR(csr, publicKey,
		func(x509Csr *x509.CertificateRequest) error {
			if x509Csr.Subject.CommonName != CommonNameControlPlaneCSR(remoteClusterIdentity.ClusterID) {
				return fmt.Errorf("invalid common name")
			}
			return nil
		},
		func(x509Csr *x509.CertificateRequest) error {
			if x509Csr.Subject.Organization[0] != OrganizationControlPlaneCSR() {
				return fmt.Errorf("invalid organization")
			}
			return nil
		})
}

// CheckCSRForResourceSlice checks a CSR for a resource slice.
func CheckCSRForResourceSlice(publicKey []byte, resourceSlice *authv1alpha1.ResourceSlice) error {
	return checkCSR(resourceSlice.Spec.CSR, publicKey,
		func(x509Csr *x509.CertificateRequest) error {
			if x509Csr.Subject.CommonName != CommonNameResourceSliceCSR(resourceSlice) {
				return fmt.Errorf("invalid common name")
			}
			return nil
		},
		func(x509Csr *x509.CertificateRequest) error {
			if x509Csr.Subject.Organization[0] != OrganizationResourceSliceCSR(resourceSlice) {
				return fmt.Errorf("invalid organization")
			}
			return nil
		})
}

func checkCSR(csr, publicKey []byte, commonName, organization CSRChecker) error {
	pemCsr, rst := pem.Decode(csr)
	if pemCsr == nil || len(rst) != 0 {
		return fmt.Errorf("invalid CSR")
	}

	x509Csr, err := x509.ParseCertificateRequest(pemCsr.Bytes)
	if err != nil {
		return err
	}

	if err := commonName(x509Csr); err != nil {
		return err
	}

	if err := organization(x509Csr); err != nil {
		return err
	}

	// if the pub key is 0-terminated, drop it
	if publicKey[len(publicKey)-1] == 0 {
		publicKey = publicKey[:len(publicKey)-1]
	}

	switch crtKey := x509Csr.PublicKey.(type) {
	case ed25519.PublicKey:
		if !bytes.Equal(crtKey, publicKey) {
			return fmt.Errorf("invalid public key")
		}
	default:
		return fmt.Errorf("invalid public key type %T", crtKey)
	}

	return nil
}

func generateCSR(key ed25519.PrivateKey, commonName, organization string) (csrBytes []byte, err error) {
	asn1Subj, err := asn1.Marshal(pkix.Name{CommonName: commonName, Organization: []string{organization}}.ToRDNSequence())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subject information: %w", err)
	}

	template := x509.CertificateRequest{
		RawSubject:         asn1Subj,
		SignatureAlgorithm: x509.PureEd25519,
	}

	csrBytes, err = x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate request: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes}), nil
}
