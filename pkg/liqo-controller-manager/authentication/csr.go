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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
)

// GenerateCSR generates a new CSR given a private key and a subject.
func GenerateCSR(key ed25519.PrivateKey, commonName string) (csrBytes []byte, err error) {
	asn1Subj, err := asn1.Marshal(pkix.Name{CommonName: commonName, Organization: []string{"liqo.io"}}.ToRDNSequence())
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

// CommonName returns a common name given the cluster identity.
func CommonName(clusterID string) string {
	return clusterID
}
