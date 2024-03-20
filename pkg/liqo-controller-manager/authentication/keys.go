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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// AuthKeysSecretName is the name of the secret containing the authentication keys.
	AuthKeysSecretName = "authentication-keys"
	// PrivateKeyField is the field key of the secret containing the private key.
	PrivateKeyField = "privateKey"
	// PublicKeyField is the field key of the secret containing the public key.
	PublicKeyField = "publicKey"
)

// InitClusterKeys initializes the authentication keys for the cluster.
// If the secret containing the keys does not exist, it generates a new pair of keys and stores them in a secret.
func InitClusterKeys(ctx context.Context, cl client.Client, liqoNamespace string) error {
	// Get secret if it exists
	var secret corev1.Secret
	err := cl.Get(ctx, client.ObjectKey{Name: AuthKeysSecretName, Namespace: liqoNamespace}, &secret)
	switch {
	case apierrors.IsNotFound(err):
		// Forge a new pair of keys.
		private, public, err := GenerateEd25519Keys()
		if err != nil {
			return fmt.Errorf("error while generating cluster authentication keys: %w", err)
		}

		// Store the keys in a secret.
		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      AuthKeysSecretName,
				Namespace: liqoNamespace,
			},
			Data: map[string][]byte{
				PrivateKeyField: private,
				PublicKeyField:  public,
			},
		}
		if err := cl.Create(ctx, &secret); err != nil {
			return fmt.Errorf("error while creating secret %s/%s: %w", liqoNamespace, AuthKeysSecretName, err)
		}
		klog.Infof("Created Secret (%s/%s) containing cluster authentication keys", liqoNamespace, AuthKeysSecretName)
	case err != nil:
		return fmt.Errorf("unable to get secret with cluster authentication keys: %w", err)
	default:
		// If secret already exists, do nothing.
		klog.V(6).Infof("Secret %s/%s already created", liqoNamespace, AuthKeysSecretName)
	}

	return nil
}
