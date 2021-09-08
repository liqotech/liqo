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

package interfaces

import (
	corev1 "k8s.io/api/core/v1"
)

// ClusterResourceInterface represents a generic subset of Broadcaster exported methods to be used instead of a direct access to
// the Broadcaster instance and get/update some cluster resources information.
type ClusterResourceInterface interface {
	// ReadResources returns all free cluster resources calculated for a given clusterID scaled by a percentage value.
	ReadResources(clusterID string) corev1.ResourceList
	// RemoveClusterID removes given clusterID from all internal structures and it will be no more valid.
	RemoveClusterID(clusterID string)
}
