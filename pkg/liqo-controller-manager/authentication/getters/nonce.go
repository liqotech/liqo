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

package getters

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/liqotech/liqo/pkg/consts"
)

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
