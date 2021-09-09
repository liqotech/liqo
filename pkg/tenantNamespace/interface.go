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

package tenantnamespace

import (
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// Manager provides the methods to handle the creation and
// the management of tenant namespaces.
type Manager interface {
	CreateNamespace(clusterID string) (*v1.Namespace, error)
	GetNamespace(clusterID string) (*v1.Namespace, error)
	BindClusterRoles(clusterID string, clusterRoles ...*rbacv1.ClusterRole) ([]*rbacv1.RoleBinding, error)
	UnbindClusterRoles(clusterID string, clusterRoles ...string) error
}
