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

package authentication

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

// CSRChecker is a function that checks a CSR.
type CSRChecker func(*x509.CertificateRequest) error

// GenerateCSRForResourceSlice generates a new CSR given a private key and a resource slice.
func GenerateCSRForResourceSlice(key ed25519.PrivateKey,
	resourceSlice *authv1beta1.ResourceSlice) (csrBytes []byte, err error) {
	return generateCSR(key, CommonNameResourceSliceCSR(resourceSlice), OrganizationResourceSliceCSR(resourceSlice))
}

// CommonNameResourceSliceCSR returns the common name for a resource slice CSR.
func CommonNameResourceSliceCSR(resourceSlice *authv1beta1.ResourceSlice) string {
	// hash the clusterID and take the first 6 chars
	// to avoid too long common names
	// this is not a security measure, just a way to keep the common name short
	// and unique

	clusterID := resourceSlice.Spec.ConsumerClusterID
	if clusterID == nil {
		// this should never happen
		panic("resource slice without consumer cluster ID")
	}

	hash := sha256.New()
	hash.Write([]byte(*clusterID))
	h := hash.Sum(nil)

	return fmt.Sprintf("%s-%x", resourceSlice.Name, h[:6])
}

// OrganizationResourceSliceCSR returns the organization for a resource slice CSR.
func OrganizationResourceSliceCSR(resourceSlice *authv1beta1.ResourceSlice) string {
	return string(*resourceSlice.Spec.ConsumerClusterID)
}

// GenerateCSRForControlPlane generates a new CSR given a private key and a subject.
func GenerateCSRForControlPlane(key ed25519.PrivateKey, clusterID liqov1beta1.ClusterID) (csrBytes []byte, err error) {
	return generateCSR(key, CommonNameControlPlaneCSR(clusterID), OrganizationControlPlaneCSR())
}

// GenerateCSRForPeerUser generates a new CSR given a private key and the clusterID from which the peering will start.
func GenerateCSRForPeerUser(key ed25519.PrivateKey, clusterID liqov1beta1.ClusterID) (csrBytes []byte, userCN string, err error) {
	userCN, err = commonNamePeerUser(clusterID)
	if err != nil {
		return nil, "", fmt.Errorf("unable to generate user CN: %w", err)
	}

	csrBytes, err = generateCSR(key, userCN, OrganizationControlPlaneCSR())
	return
}

// commonNamePeerUser returns the common name for the user creating the peering. To avoid reuses of the same name, a suffix is added.
func commonNamePeerUser(clusterID liqov1beta1.ClusterID) (string, error) {
	randSuffix := make([]byte, 16)
	if _, err := rand.Read(randSuffix); err != nil {
		return "", err
	}
	return fmt.Sprintf("liqo-peer-user-%s-%x", clusterID, randSuffix), nil
}

// CommonNameControlPlaneCSR returns the common name for a control plane CSR.
func CommonNameControlPlaneCSR(clusterID liqov1beta1.ClusterID) string {
	return string(clusterID)
}

// OrganizationControlPlaneCSR returns the organization for a control plane CSR.
func OrganizationControlPlaneCSR() string {
	return "liqo.io"
}

// IsControlPlaneUser checks if a user is a control plane user.
func IsControlPlaneUser(groups []string) bool {
	for _, group := range groups {
		if group == OrganizationControlPlaneCSR() {
			return true
		}
	}
	return false
}

// CheckCSRForControlPlane checks a CSR for a control plane.
func CheckCSRForControlPlane(csr, publicKey []byte, remoteClusterID liqov1beta1.ClusterID) error {
	return checkCSR(csr, publicKey, true,
		func(x509Csr *x509.CertificateRequest) error {
			if x509Csr.Subject.CommonName != CommonNameControlPlaneCSR(remoteClusterID) {
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
func CheckCSRForResourceSlice(publicKey []byte, resourceSlice *authv1beta1.ResourceSlice, checkPublicKey bool) error {
	return checkCSR(resourceSlice.Spec.CSR, publicKey, checkPublicKey,
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

func checkCSR(csr, publicKey []byte, checkPublicKey bool, commonName, organization CSRChecker) error {
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

	if checkPublicKey {
		// Check the length of the public key and return an error if invalid
		if len(publicKey) == 0 {
			return fmt.Errorf("invalid public key")
		}

		// if the pub key is 0-terminated, drop it
		if publicKey[len(publicKey)-1] == 0 {
			publicKey = publicKey[:len(publicKey)-1]
		}

		// Check that the public key used the expected algorithm and verify that the CSR has been
		// signed with the key provided by the peer at peering time.
		switch crtKey := x509Csr.PublicKey.(type) {
		case ed25519.PublicKey:
			if !bytes.Equal(crtKey, publicKey) {
				return fmt.Errorf("invalid public key")
			}
		default:
			return fmt.Errorf("invalid public key type %T", crtKey)
		}
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
