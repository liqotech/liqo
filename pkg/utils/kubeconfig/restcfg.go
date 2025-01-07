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

package kubeconfig

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/liqotech/liqo/pkg/consts"
)

// BuildConfigFromSecret builds a REST config from a secret containing a kubeconfig.
func BuildConfigFromSecret(secret *corev1.Secret) (*rest.Config, error) {
	kubeconfig, ok := secret.Data[consts.KubeconfigSecretField]
	if !ok {
		return nil, fmt.Errorf("key %v not found in secret %v/%v", consts.KubeconfigSecretField, secret.Namespace, secret.Name)
	}

	return clientcmd.RESTConfigFromKubeConfig(kubeconfig)
}
