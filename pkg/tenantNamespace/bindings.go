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
	"maps"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// add the bindings for the remote clusterid for the given ClusterRoles
// This method creates RoleBindings in the Tenant Namespace for a remote identity.
func (nm *tenantNamespaceManager) BindClusterRoles(ctx context.Context, cluster liqov1beta1.ClusterID,
	owner metav1.Object, clusterRoles ...*rbacv1.ClusterRole) ([]*rbacv1.RoleBinding, error) {
	namespace, err := nm.GetNamespace(ctx, cluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	bindings := make([]*rbacv1.RoleBinding, len(clusterRoles))
	for i, clusterRole := range clusterRoles {
		bindings[i], err = nm.bindClusterRole(ctx, cluster, owner, namespace, clusterRole)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
	}
	return bindings, nil
}

// remove the bindings for the remote clusterid for the given ClusterRoles
// This method deletes RoleBindings in the Tenant Namespace for a remote identity.
func (nm *tenantNamespaceManager) UnbindClusterRoles(ctx context.Context, cluster liqov1beta1.ClusterID, clusterRoles ...*rbacv1.ClusterRole) error {
	namespace, err := nm.GetNamespace(ctx, cluster)
	if err != nil {
		klog.Error(err)
		return err
	}

	for i := range clusterRoles {
		if err = nm.unbindClusterRole(ctx, namespace, clusterRoles[i].Name); err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

// create a RoleBinding for the given clusterid in the given Namespace.
func (nm *tenantNamespaceManager) bindClusterRole(ctx context.Context, cluster liqov1beta1.ClusterID,
	owner metav1.Object, namespace *v1.Namespace, clusterRole *rbacv1.ClusterRole) (*rbacv1.RoleBinding, error) {
	name := getRoleBindingName(clusterRole.Name)
	labels := map[string]string{
		consts.K8sAppManagedByKey: consts.LiqoAppLabelValue,
		consts.RemoteClusterID:    string(cluster),
	}
	maps.Copy(labels, resource.GetGlobalLabels())

	annotations := resource.GetGlobalAnnotations()

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace.Name,
			Labels:      labels,
			Annotations: annotations,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.UserKind,
				APIGroup: rbacv1.GroupName,
				Name:     string(cluster),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
	}

	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, rb, nm.scheme); err != nil {
			return nil, err
		}
	}

	resource.AddGlobalLabels(rb)
	resource.AddGlobalAnnotations(rb)

	rb, err := nm.client.RbacV1().RoleBindings(namespace.Name).Create(ctx, rb, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nm.client.RbacV1().RoleBindings(namespace.Name).Get(ctx, name, metav1.GetOptions{})
	}
	return rb, err
}

// delete a RoleBinding in the given Namespace.
func (nm *tenantNamespaceManager) unbindClusterRole(ctx context.Context, namespace *v1.Namespace, clusterRole string) error {
	name := getRoleBindingName(clusterRole)
	return client.IgnoreNotFound(nm.client.RbacV1().RoleBindings(namespace.Name).Delete(ctx, name, metav1.DeleteOptions{}))
}

func getRoleBindingName(clusterRoleName string) string {
	return roleBindingRoot + "-" + clusterRoleName
}

func getClusterRoleBindingName(clusterRoleName string, cluster liqov1beta1.ClusterID) string {
	return roleBindingRoot + "-" + clusterRoleName + "-" + string(cluster)
}

// BindClusterRolesClusterWide creates ClusterRoleBindings for the given ClusterRoles.
func (nm *tenantNamespaceManager) BindClusterRolesClusterWide(ctx context.Context, cluster liqov1beta1.ClusterID,
	owner metav1.Object, clusterRoles ...*rbacv1.ClusterRole) ([]*rbacv1.ClusterRoleBinding, error) {
	var err error
	bindings := make([]*rbacv1.ClusterRoleBinding, len(clusterRoles))
	for i, clusterRole := range clusterRoles {
		bindings[i], err = nm.bindClusterRoleClusterWide(ctx, cluster, owner, clusterRole)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
	}
	return bindings, nil
}

func (nm *tenantNamespaceManager) bindClusterRoleClusterWide(ctx context.Context, cluster liqov1beta1.ClusterID,
	owner metav1.Object, clusterRole *rbacv1.ClusterRole) (*rbacv1.ClusterRoleBinding, error) {
	name := getClusterRoleBindingName(clusterRole.Name, cluster)
	labels := map[string]string{
		consts.K8sAppManagedByKey: consts.LiqoAppLabelValue,
		consts.RemoteClusterID:    string(cluster),
	}
	maps.Copy(labels, resource.GetGlobalLabels())

	annotations := resource.GetGlobalAnnotations()
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.GroupKind,
				APIGroup: rbacv1.GroupName,
				Name:     string(cluster),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
	}

	if owner != nil {
		if err := controllerutil.SetControllerReference(owner, crb, nm.scheme); err != nil {
			return nil, err
		}
	}

	resource.AddGlobalLabels(crb)
	resource.AddGlobalAnnotations(crb)

	crb, err := nm.client.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nm.client.RbacV1().ClusterRoleBindings().Get(ctx, name, metav1.GetOptions{})
	}
	return crb, err
}

// UnbindClusterRolesClusterWide deletes ClusterRoleBindings for the given ClusterRoles.
func (nm *tenantNamespaceManager) UnbindClusterRolesClusterWide(ctx context.Context, cluster liqov1beta1.ClusterID,
	clusterRoles ...*rbacv1.ClusterRole) error {
	for i := range clusterRoles {
		if err := nm.unbindClusterRoleClusterWide(ctx, clusterRoles[i].Name, cluster); err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

func (nm *tenantNamespaceManager) unbindClusterRoleClusterWide(ctx context.Context, clusterRole string,
	cluster liqov1beta1.ClusterID) error {
	name := getClusterRoleBindingName(clusterRole, cluster)
	return client.IgnoreNotFound(nm.client.RbacV1().ClusterRoleBindings().Delete(ctx, name, metav1.DeleteOptions{}))
}
