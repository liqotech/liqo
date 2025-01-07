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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// ExtractKeyFromSecretRef extracts the public key data of a secret from a secret reference.
func ExtractKeyFromSecretRef(ctx context.Context, cl client.Client, secretRef *corev1.ObjectReference) ([]byte, error) {
	var secret corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: secretRef.Namespace}, &secret); err != nil {
		return nil, err
	}
	key, ok := secret.Data[consts.PublicKeyField]
	if !ok {
		return nil, fmt.Errorf("secret %q does not contain %s field", client.ObjectKeyFromObject(&secret), consts.PublicKeyField)
	}
	return key, nil
}

// GetGatewaySecretReference returns the secret reference of a gateway.
func GetGatewaySecretReference(ctx context.Context, cl client.Client, namespace, gatewayName, gatewayType string) (*corev1.ObjectReference, error) {
	switch gatewayType {
	case consts.GatewayTypeServer:
		var gwServer networkingv1beta1.GatewayServer
		if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: gatewayName}, &gwServer); err != nil {
			return nil, err
		}
		return gwServer.Status.SecretRef, nil
	case consts.GatewayTypeClient:
		var gwClient networkingv1beta1.GatewayClient
		if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: gatewayName}, &gwClient); err != nil {
			return nil, err
		}
		return gwClient.Status.SecretRef, nil
	default:
		return nil, fmt.Errorf("unable to forge PublicKey: invalid gateway type %q", gatewayType)
	}
}
