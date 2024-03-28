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

package noncesigner

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// generateSecretNonceName generates the name of the secret containing the nonce for the provider cluster given the remote consumer cluster.
func generateSecretNonceName(remoteCluster *discoveryv1alpha1.ClusterIdentity) string {
	return "nonce-" + remoteCluster.ClusterName
}

// CreateSecretNonceConsumer creates a secret containing the nonce for the consumer cluster given the remote provider cluster.
func CreateSecretNonceConsumer(ctx context.Context, cl client.Client, tenantNamespace, nonce string,
	remoteCluster *discoveryv1alpha1.ClusterIdentity) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateSecretNonceName(remoteCluster),
			Namespace: tenantNamespace,
			Labels: map[string]string{
				consts.NonceSecretLabelKey: consts.NonceSecretConsumerLabelValue,
				consts.RemoteClusterID:     remoteCluster.ClusterID,
			},
		},
		Data: map[string][]byte{
			consts.NonceSecretField: []byte(nonce),
		},
	}

	if err := cl.Create(ctx, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

// GetSecretNonceConsumer retrieves the secret containing the nonce for the consumer cluster given the remote provider cluster.
func GetSecretNonceConsumer(ctx context.Context, cl client.Client, tenantNamespace, remoteClusterID string) (*corev1.Secret, error) {
	var secrets corev1.SecretList
	if err := cl.List(ctx, &secrets, client.InNamespace(tenantNamespace), client.MatchingLabels{
		consts.NonceSecretLabelKey: consts.NonceSecretConsumerLabelValue,
		consts.RemoteClusterID:     remoteClusterID,
	}); err != nil {
		return nil, err
	}

	switch len(secrets.Items) {
	case 0:
		return nil, errors.NewNotFound(corev1.Resource("secrets"), "nonce secret")
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
