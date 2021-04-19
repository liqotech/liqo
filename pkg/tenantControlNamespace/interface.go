package tenantControlNamespace

import (
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

type TenantControlNamespaceManager interface {
	CreateNamespace(clusterID string) (*v1.Namespace, error)
	GetNamespace(clusterID string) (*v1.Namespace, error)
	BindClusterRoles(clusterID string, clusterRoles ...*rbacv1.ClusterRole) ([]*rbacv1.RoleBinding, error)
	UnbindClusterRoles(clusterID string, clusterRoles ...string) error
}
