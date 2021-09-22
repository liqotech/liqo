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

package foreignclusteroperator

import (
	"context"

	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// ensureLocalTenantNamespace creates the LocalTenantNamespace for the given ForeignCluster, if it is not yet present.
func (r *ForeignClusterReconciler) ensureLocalTenantNamespace(
	ctx context.Context, foreignCluster *v1alpha1.ForeignCluster) error {
	namespace, err := r.NamespaceManager.CreateNamespace(foreignCluster.Spec.ClusterIdentity.ClusterID)
	if err != nil {
		klog.Error(err)
		return err
	}

	foreignCluster.Status.TenantNamespace.Local = namespace.Name
	return nil
}
