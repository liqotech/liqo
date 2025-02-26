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
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

// Manager provides the methods to handle the creation and
// the management of tenant namespaces.
type Manager interface {
	CreateNamespace(ctx context.Context, cluster liqov1beta1.ClusterID) (*corev1.Namespace, error)
	ForgeNamespace(cluster liqov1beta1.ClusterID, name *string) *corev1.Namespace
	GetNamespace(ctx context.Context, cluster liqov1beta1.ClusterID) (*corev1.Namespace, error)
	BindClusterRoles(ctx context.Context, cluster liqov1beta1.ClusterID,
		owner metav1.Object, clusterRoles ...*rbacv1.ClusterRole) ([]*rbacv1.RoleBinding, error)
	UnbindClusterRoles(ctx context.Context, cluster liqov1beta1.ClusterID, clusterRoles ...*rbacv1.ClusterRole) error
	BindClusterRolesClusterWide(ctx context.Context, cluster liqov1beta1.ClusterID,
		owner metav1.Object, clusterRoles ...*rbacv1.ClusterRole) ([]*rbacv1.ClusterRoleBinding, error)
	UnbindClusterRolesClusterWide(ctx context.Context, cluster liqov1beta1.ClusterID, clusterRoles ...*rbacv1.ClusterRole) error
}
