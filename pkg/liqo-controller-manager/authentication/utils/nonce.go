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

package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	authgetters "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/getters"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// EnsureNonceSecret ensures that a nonce secret exists in the tenant namespace.
func EnsureNonceSecret(ctx context.Context, cl client.Client,
	remoteClusterID, tenantNamespace string) error {
	nonce := forge.Nonce(tenantNamespace)
	_, err := controllerutil.CreateOrUpdate(ctx, cl, nonce, func() error {
		return forge.MutateNonce(nonce, remoteClusterID)
	})
	if err != nil {
		return fmt.Errorf("unable to create nonce secret: %w", err)
	}
	return nil
}

// EnsureSignedNonceSecret ensures that a signed nonce secret exists in the tenant namespace.
// If nonce is not provided, get it from the secret in the tenant namespace and raise an error if the secret does not exist.
// If nonce is provided, create nonce secret in the tenant namespace and wait for it to be signed. Raise an error if there is
// already a nonce secret in the tenant namespace.
func EnsureSignedNonceSecret(ctx context.Context, cl client.Client,
	remoteClusterID, tenantNamespace string, nonce *string) error {
	nonceSecret, err := getters.GetSignedNonceSecretByClusterID(ctx, cl, remoteClusterID)
	switch {
	case errors.IsNotFound(err):
		// Secret not found. Create it given the provided nonce.
		if nonce == nil || *nonce == "" {
			return fmt.Errorf("nonce not provided and no nonce secret found")
		}
		secret := forge.SignedNonce(remoteClusterID, tenantNamespace, *nonce)
		if err := cl.Create(ctx, secret); err != nil {
			return fmt.Errorf("unable to create nonce secret: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("unable to get nonce secret: %w", err)
	default:
		// Secret already exists.
		existingNonce, err := authgetters.GetNonceFromSecret(nonceSecret)
		if err != nil {
			return fmt.Errorf("unable to extract nonce data from secret %s/%s: %w", nonceSecret.Namespace, nonceSecret.Name, err)
		}
		// If nonce is provided, check if it is the same of the one in the secret. Otherwise, raise an error.
		if nonce != nil && *nonce != string(existingNonce) {
			return fmt.Errorf("nonce secret already exists with a different nonce: %s", *nonce)
		}
		return nil
	}
}

// RetrieveNonce retrieves the nonce from the secret in the tenant namespace.
func RetrieveNonce(ctx context.Context, cl client.Client, remoteClusterID string) ([]byte, error) {
	nonce, err := getters.GetNonceSecretByClusterID(ctx, cl, remoteClusterID)
	if err != nil {
		return nil, fmt.Errorf("unable to get nonce secret: %w", err)
	}

	return authgetters.GetNonceFromSecret(nonce)
}

// RetrieveSignedNonce retrieves the signed nonce from the secret in the tenant namespace.
func RetrieveSignedNonce(ctx context.Context, cl client.Client, remoteClusterID string) ([]byte, error) {
	secret, err := getters.GetSignedNonceSecretByClusterID(ctx, cl, remoteClusterID)
	if err != nil {
		return nil, fmt.Errorf("unable to get signed nonce secret: %w", err)
	}

	return authgetters.GetSignedNonceFromSecret(secret)
}
