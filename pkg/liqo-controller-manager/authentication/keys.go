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
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// GenerateEd25519Keys returns a new pair of private and public keys in PEM format.
// Keys are generated using the Ed25519 signature algorithm and encoded in PEM format.
func GenerateEd25519Keys() (privateKey, publicKey []byte, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes})

	return privateKeyPEM, publicKeyPEM, nil
}

// GenerateRSAKeys returns a new pair of RSA private and public keys in PEM format.
// Keys are generated using RSA 2048 bits and encoded in PEM format.
func GenerateRSAKeys() (privateKey, publicKey []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA private key: %w", err)
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal RSA private key: %w", err)
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal RSA public key: %w", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes})

	return privateKeyPEM, publicKeyPEM, nil
}

// SignNonce signs a nonce using the provided private key. The private key can be
// ed25519.PrivateKey, *rsa.PrivateKey, or *ecdsa.PrivateKey. For RSA/ECDSA the nonce
// is hashed with SHA-256 before signing.
func SignNonce(priv crypto.PrivateKey, nonce []byte) ([]byte, error) {
	switch k := priv.(type) {
	case ed25519.PrivateKey:
		sig := ed25519.Sign(k, nonce)
		return sig, nil
	case *rsa.PrivateKey:
		sum := sha256.Sum256(nonce)
		sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
		if err != nil {
			return nil, fmt.Errorf("rsa sign failed: %w", err)
		}
		return sig, nil
	case *ecdsa.PrivateKey:
		sum := sha256.Sum256(nonce)
		sig, err := ecdsa.SignASN1(rand.Reader, k, sum[:])
		if err != nil {
			return nil, fmt.Errorf("ecdsa sign failed: %w", err)
		}
		return sig, nil
	default:
		return nil, fmt.Errorf("unsupported private key type %T", priv)
	}
}

// VerifyNonce verifies the signature of a nonce using the PKIX-encoded public key bytes of the cluster.
// The public key can be Ed25519, RSA, or ECDSA.
func VerifyNonce(publicKey crypto.PublicKey, nonce, signature []byte) (bool, error) {
	switch pk := publicKey.(type) {
	case ed25519.PublicKey:
		return ed25519.Verify(pk, nonce, signature), nil
	case *rsa.PublicKey:
		sum := sha256.Sum256(nonce)
		if err := rsa.VerifyPKCS1v15(pk, crypto.SHA256, sum[:], signature); err != nil {
			return false, nil
		}
		return true, nil
	case *ecdsa.PublicKey:
		sum := sha256.Sum256(nonce)
		if ecdsa.VerifyASN1(pk, sum[:], signature) {
			return true, nil
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported public key type %T", publicKey)
	}
}

// InitClusterKeys initializes the authentication keys for the cluster.
// If the secret containing the keys does not exist, it generates a new pair of keys and stores them in a secret.
// If tlsCompatibilityMode is true, RSA keys are generated instead of Ed25519.
func InitClusterKeys(ctx context.Context, cl client.Client, liqoNamespace string, tlsCompatibilityMode bool) error {
	// Get secret if it exists
	var secret corev1.Secret
	err := cl.Get(ctx, client.ObjectKey{Name: consts.AuthKeysSecretName, Namespace: liqoNamespace}, &secret)
	switch {
	case apierrors.IsNotFound(err):
		// Forge a new pair of keys.
		var private, public []byte
		if tlsCompatibilityMode {
			private, public, err = GenerateRSAKeys()
		} else {
			private, public, err = GenerateEd25519Keys()
		}
		if err != nil {
			return fmt.Errorf("error while generating cluster authentication keys: %w", err)
		}

		// Store the keys in a secret.
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      consts.AuthKeysSecretName,
				Namespace: liqoNamespace,
			},
			Data: map[string][]byte{
				consts.PrivateKeyField: private,
				consts.PublicKeyField:  public,
			},
		}
		if _, err := resource.CreateOrUpdate(ctx, cl, &secret, func() error {
			return nil
		}); err != nil {
			return fmt.Errorf("error while creating secret %s/%s: %w", liqoNamespace, consts.AuthKeysSecretName, err)
		}
		klog.Infof("Created Secret (%s/%s) containing cluster authentication keys", liqoNamespace, consts.AuthKeysSecretName)
	case err != nil:
		return fmt.Errorf("unable to get secret with cluster authentication keys: %w", err)
	default:
		// If secret already exists, do nothing.
		klog.V(6).Infof("Secret %s/%s already created", liqoNamespace, consts.AuthKeysSecretName)
	}

	return nil
}

// GetClusterKeys retrieves the private and public keys of the cluster from the secret.
// It returns the private key as crypto.PrivateKey and the public key as PKIX-encoded bytes.
func GetClusterKeys(ctx context.Context, cl client.Client, liqoNamespace string) (crypto.PrivateKey, []byte, error) {
	var secret corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Name: consts.AuthKeysSecretName, Namespace: liqoNamespace}, &secret); err != nil {
		return nil, nil, fmt.Errorf("unable to get secret with cluster authentication keys: %w", err)
	}

	// Get the private key from the secret.
	privateKey, found := secret.Data[consts.PrivateKeyField]
	if !found {
		return nil, nil, fmt.Errorf("private key not found in secret %s/%s", liqoNamespace, consts.AuthKeysSecretName)
	}

	privateKeyPEM, _ := pem.Decode(privateKey)
	if privateKeyPEM == nil {
		return nil, nil, fmt.Errorf("failed to decode private key in PEM format")
	}
	priv, err := x509.ParsePKCS8PrivateKey(privateKeyPEM.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Get the public key from the secret.
	publicKey, found := secret.Data[consts.PublicKeyField]
	if !found {
		return nil, nil, fmt.Errorf("public key not found in secret %s/%s", liqoNamespace, consts.AuthKeysSecretName)
	}

	// The public key is stored in PEM format with PKIX-encoded bytes. Return the DER bytes for portability.
	publicKeyPEM, _ := pem.Decode(publicKey)
	if publicKeyPEM == nil {
		return nil, nil, fmt.Errorf("failed to decode public key in PEM format")
	}

	return priv, publicKeyPEM.Bytes, nil
}

// GetClusterKeysPEM retrieves the private and public keys of the cluster from the secret and encoded in PEM format.
func GetClusterKeysPEM(ctx context.Context, cl client.Client, liqoNamespace string) (privateKey, publicKey []byte, err error) {
	var secret corev1.Secret
	if err = cl.Get(ctx, client.ObjectKey{Name: consts.AuthKeysSecretName, Namespace: liqoNamespace}, &secret); err != nil {
		return nil, nil, fmt.Errorf("unable to get secret with cluster authentication keys: %w", err)
	}

	// Get the private key from the secret.
	privateKey, found := secret.Data[consts.PrivateKeyField]
	if !found {
		return nil, nil, fmt.Errorf("private key not found in secret %s/%s", liqoNamespace, consts.AuthKeysSecretName)
	}

	// Get the public key from the secret.
	publicKey, found = secret.Data[consts.PublicKeyField]
	if !found {
		return nil, nil, fmt.Errorf("public key not found in secret %s/%s", liqoNamespace, consts.AuthKeysSecretName)
	}

	return privateKey, publicKey, nil
}
