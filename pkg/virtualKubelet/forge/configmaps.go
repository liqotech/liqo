// Copyright 2019-2023 The Liqo Authors
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

// RootCAConfigMapName is the name of the configmap containing the root CA.
const RootCAConfigMapName = "kube-root-ca.crt"

// RemoteConfigMap forges the apply patch for the reflected configmap, given the local one.
func RemoteConfigMap(local *corev1.ConfigMap, targetNamespace string) *corev1apply.ConfigMapApplyConfiguration {
	applyConfig := corev1apply.ConfigMap(RemoteConfigMapName(local.GetName()), targetNamespace).
		WithLabels(local.GetLabels()).WithLabels(ReflectionLabels()).
		WithAnnotations(local.GetAnnotations()).
		WithBinaryData(local.BinaryData).
		WithData(local.Data)

	if local.Immutable != nil {
		applyConfig = applyConfig.WithImmutable(*local.Immutable)
	}

	return applyConfig
}

// LocalConfigMapName returns the local configmap name corresponding to a remote one, accounting for the root CA.
func LocalConfigMapName(remote string) string {
	if remote == RemoteConfigMapName(RootCAConfigMapName) {
		return RootCAConfigMapName
	}

	return remote
}

// RemoteConfigMapName forges the name for the reflected configmap, remapping the one of the root CA to prevent collisions.
func RemoteConfigMapName(local string) string {
	if local == RootCAConfigMapName {
		return RootCAConfigMapName + "." + LocalCluster.ClusterID[0:5]
	}

	return local
}
