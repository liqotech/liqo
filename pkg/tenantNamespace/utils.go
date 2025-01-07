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

package tenantnamespace

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// GetClusterIDFromTenantNamespace returns the cluster ID of the cluster that owns the given tenant namespace.
func GetClusterIDFromTenantNamespace(namespace *corev1.Namespace) (liqov1beta1.ClusterID, error) {
	if namespace.Labels == nil {
		return "", fmt.Errorf("namespace %s has no labels", namespace.Name)
	}

	if _, ok := namespace.Labels[consts.TenantNamespaceLabel]; !ok {
		return "", fmt.Errorf("namespace %s is not a tenant namespace", namespace.Name)
	}

	if _, ok := namespace.Labels[consts.RemoteClusterID]; !ok {
		return "", fmt.Errorf("namespace %s has no cluster ID label", namespace.Name)
	}

	return liqov1beta1.ClusterID(namespace.Labels[consts.RemoteClusterID]), nil
}
