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

// generateSignedNonceSecretName generates the name of the Secret object to store the signed nonce.
func generateSignedNonceSecretName() string {
	return "liqo-signed-nonce"
}

// SignedNonce creates a new Secret object to store the nonce signed by the consumer cluster.
func SignedNonce(remoteClusterID liqov1beta1.ClusterID, tenantNamespace, nonce string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateSignedNonceSecretName(),
			Namespace: tenantNamespace,
			Labels: map[string]string{
				consts.SignedNonceSecretLabelKey: "true",
				consts.RemoteClusterID:           string(remoteClusterID),
			},
		},
		StringData: map[string]string{
			consts.NonceSecretField: nonce,
		},
	}
}
