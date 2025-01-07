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

package forge

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// generateNonceSecretName generates the name of the Secret object to store the nonce.
func generateNonceSecretName() string {
	return "liqo-nonce"
}

// Nonce creates a new Secret object to store the nonce.
func Nonce(tenantNamespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateNonceSecretName(),
			Namespace: tenantNamespace,
		},
	}
}

// MutateNonce sets the nonce labels and data.
func MutateNonce(nonce *corev1.Secret, remoteClusterID liqov1beta1.ClusterID) error {
	if nonce.Labels == nil {
		nonce.Labels = make(map[string]string)
	}

	nonce.Labels[consts.NonceSecretLabelKey] = "true"
	nonce.Labels[consts.RemoteClusterID] = string(remoteClusterID)

	return nil
}
