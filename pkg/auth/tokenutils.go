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

package auth

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// TokenSecretName is the name of the secret containing the authentication token for the local cluster.
	TokenSecretName = "auth-token"
)

// GetToken retrieves the token for the local cluster.
func GetToken(ctx context.Context, c client.Client, namespace string) (string, error) {
	var secret v1.Secret
	if err := c.Get(ctx, types.NamespacedName{
		Name:      TokenSecretName,
		Namespace: namespace,
	}, &secret); err != nil {
		return "", err
	}

	return GetTokenFromSecret(&secret)
}

// GetTokenFromSecret retrieves the token for the local cluster given its secret.
func GetTokenFromSecret(secret *v1.Secret) (string, error) {
	v, ok := secret.Data["token"]
	if !ok {
		err := fmt.Errorf("invalid secret %v/%v: does not contain a valid token",
			secret.GetNamespace(), secret.GetName())
		klog.Error(err)
		return "", err
	}
	return string(v), nil
}
