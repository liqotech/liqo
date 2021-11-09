// Copyright 2019-2021 The Liqo Authors
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

	return applyConfig
}
