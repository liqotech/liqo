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

package webhookConfiguration

import (
	"bytes"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

type SecretsType struct {
	caPEM         *bytes.Buffer
	serverCertPEM *bytes.Buffer
	serverKeyPEM  *bytes.Buffer
}

func NewSecrets(name string) (*SecretsType, error) {
	nameComponents := strings.Split(name, ".")

	secrets := &SecretsType{
		caPEM:         new(bytes.Buffer),
		serverCertPEM: new(bytes.Buffer),
		serverKeyPEM:  new(bytes.Buffer),
	}

	klog.Info("creating CA")
	// CA config
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(rand.Int63()),
		Subject: pkix.Name{
			Organization: []string{"liqo.io"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// CA private key
	caPublicKey, caPrivKey, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		return nil, err
	}

	klog.Info("self-signing CA")
	// Self signed CA certificate
	caBytes, err := x509.CreateCertificate(cryptorand.Reader, ca, ca, caPublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}

	dnsNames := []string{nameComponents[0],
		fmt.Sprintf("%s.%s", nameComponents[0], nameComponents[1]),
		fmt.Sprintf("%s.%s.svc", nameComponents[0], nameComponents[1])}
	commonName := name

	klog.Info("creating server certificate")
	// server cert config
	cert := &x509.Certificate{
		DNSNames:     dnsNames,
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"liqo.io"},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	// server private key
	serverPublicKey, serverPrivKey, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		return nil, err
	}

	klog.Info("signing server certificate with CA")
	// sign the server cert
	serverCertBytes, err := x509.CreateCertificate(cryptorand.Reader, cert, ca, serverPublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}

	klog.Info("encoding CA")
	// PEM encode CA cert
	if err = pem.Encode(secrets.caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	}); err != nil {
		return nil, err
	}

	klog.Info("encoding server cert")
	// PEM encode the server cert and key
	if err = pem.Encode(secrets.serverCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertBytes,
	}); err != nil {
		return nil, err
	}

	klog.Info("encoding server key")
	keyBytes, err := x509.MarshalPKCS8PrivateKey(serverPrivKey)
	if err != nil {
		klog.Error("Failed to marshal private key: %w", err)
		return nil, err
	}

	// PEM encode the server and key
	if err = pem.Encode(secrets.serverKeyPEM, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return nil, err
	}

	return secrets, nil
}

func (s *SecretsType) WriteFiles(certFile, keyFile string) error {
	err := WriteFile(certFile, s.serverCertPEM)
	if err != nil {
		return err
	}

	err = WriteFile(keyFile, s.serverKeyPEM)
	if err != nil {
		return err
	}

	return nil
}

// WriteFile writes data in the file at the given path.
func WriteFile(filepath string, sCert *bytes.Buffer) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(sCert.Bytes())
	if err != nil {
		return err
	}
	return nil
}

// ServerCertPEM returns the server certificate in PEM format.
func (s *SecretsType) ServerCertPEM() []byte {
	return s.serverCertPEM.Bytes()
}

// ServerKeyPEM returns the server key in PEM format.
func (s *SecretsType) ServerKeyPEM() []byte {
	return s.serverKeyPEM.Bytes()
}

func (s *SecretsType) CAPEM() string {
	return base64.StdEncoding.EncodeToString(s.caPEM.Bytes())
}
