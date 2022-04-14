// Copyright 2019-2022 The Liqo Authors
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
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
)

// RemoteSecret forges the apply patch for the reflected secret, given the local one.
func RemoteSecret(local *corev1.Secret, targetNamespace string) *corev1apply.SecretApplyConfiguration {
	applyConfig := corev1apply.Secret(local.GetName(), targetNamespace).
		WithLabels(local.GetLabels()).WithLabels(ReflectionLabels()).
		WithAnnotations(local.GetAnnotations()).
		WithData(local.Data).
		WithType(local.Type)

	if local.Immutable != nil {
		applyConfig = applyConfig.WithImmutable(*local.Immutable)
	}

	// It is not possible to create a ServiceAccountToken secret if the corresponding
	// service account does not exist, hence it is mutated to an opaque secret.
	// In addition, we also add a label with the service account name, for easier retrieval.
	if local.Type == corev1.SecretTypeServiceAccountToken {
		applyConfig = applyConfig.WithType(corev1.SecretTypeOpaque).
			WithLabels(map[string]string{corev1.ServiceAccountNameKey: local.Annotations[corev1.ServiceAccountNameKey]})
	}

	return applyConfig
}
