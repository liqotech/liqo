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

package noncesignercontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
)

const secretSignedNonceName = "liqo-signed-nonce" //nolint:gosec // this is not a credential

// CreateSignedNonceSecret creates the secret containing the nonce and its signature.
// The nonce is signed by the nonce signer controller in the consumer cluster.
func CreateSignedNonceSecret(ctx context.Context, cl client.Client, remoteClusterID, tenantNamespace, nonce string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretSignedNonceName,
			Namespace: tenantNamespace,
			Labels: map[string]string{
				consts.SignedNonceSecretLabelKey: "true",
				consts.RemoteClusterID:           remoteClusterID,
			},
		},
		StringData: map[string]string{
			consts.NonceSecretField: nonce,
		},
	}

	if err := cl.Create(ctx, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

// GetSignedNonceSecret retrieves the secret containing the nonce signed by the consumer cluster.
func GetSignedNonceSecret(ctx context.Context, cl client.Client, remoteClusterID string) (*corev1.Secret, error) {
	var secrets corev1.SecretList
	if err := cl.List(ctx, &secrets, client.MatchingLabels{
		consts.SignedNonceSecretLabelKey: "true",
		consts.RemoteClusterID:           remoteClusterID,
	}); err != nil {
		return nil, err
	}

	switch len(secrets.Items) {
	case 0:
		return nil, errors.NewNotFound(corev1.Resource(string(corev1.ResourceSecrets)), remoteClusterID)
	case 1:
		return &secrets.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple nonce secrets found for remote cluster %q", remoteClusterID)
	}
}

// GetNonceFromSecret retrieves the nonce from the given secret.
func GetNonceFromSecret(secret *corev1.Secret) ([]byte, error) {
	nonce, found := secret.Data[consts.NonceSecretField]
	if !found {
		return nil, fmt.Errorf("nonce not found")
	} else if len(nonce) == 0 {
		return nil, fmt.Errorf("empty nonce found")
	}

	return nonce, nil
}

// GetSignedNonceFromSecret retrieves the signed nonce from the given secret.
func GetSignedNonceFromSecret(secret *corev1.Secret) ([]byte, error) {
	signedNonce, found := secret.Data[consts.SignedNonceSecretField]
	if !found {
		return nil, fmt.Errorf("signed nonce not found")
	} else if len(signedNonce) == 0 {
		return nil, fmt.Errorf("empty signed nonce found")
	}

	return signedNonce, nil
}
