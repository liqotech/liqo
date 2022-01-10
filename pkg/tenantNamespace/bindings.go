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

package tenantnamespace

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// add the bindings for the remote clusterid for the given ClusterRoles
// This method creates RoleBindings in the Tenant Namespace for a remote identity.
func (nm *tenantNamespaceManager) BindClusterRoles(cluster discoveryv1alpha1.ClusterIdentity,
	clusterRoles ...*rbacv1.ClusterRole) ([]*rbacv1.RoleBinding, error) {
	namespace, err := nm.GetNamespace(cluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	bindings := make([]*rbacv1.RoleBinding, len(clusterRoles))
	for i, clusterRole := range clusterRoles {
		bindings[i], err = nm.bindClusterRole(cluster, namespace, clusterRole)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
	}
	return bindings, nil
}

// remove the bindings for the remote clusterid for the given ClusterRoles
// This method deletes RoleBindings in the Tenant Namespace for a remote identity.
func (nm *tenantNamespaceManager) UnbindClusterRoles(cluster discoveryv1alpha1.ClusterIdentity, clusterRoles ...string) error {
	namespace, err := nm.GetNamespace(cluster)
	if err != nil {
		klog.Error(err)
		return err
	}

	for _, clusterRole := range clusterRoles {
		if err = nm.unbindClusterRole(namespace, clusterRole); err != nil {
			klog.Error(err)
			return err
		}
	}
	return nil
}

// create a RoleBinding for the given clusterid in the given Namespace.
func (nm *tenantNamespaceManager) bindClusterRole(cluster discoveryv1alpha1.ClusterIdentity,
	namespace *v1.Namespace, clusterRole *rbacv1.ClusterRole) (*rbacv1.RoleBinding, error) {
	ownerRef := metav1.OwnerReference{
		APIVersion: rbacv1.SchemeGroupVersion.String(),
		Kind:       "ClusterRole",
		Name:       clusterRole.Name,
		UID:        clusterRole.UID,
	}

	name := getRoleBindingName(clusterRole.Name)

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace.Name,
			OwnerReferences: []metav1.OwnerReference{
				ownerRef,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.UserKind,
				APIGroup: rbacv1.GroupName,
				Name:     cluster.ClusterID,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
	}

	rb, err := nm.client.RbacV1().RoleBindings(namespace.Name).Create(context.TODO(), rb, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nm.client.RbacV1().RoleBindings(namespace.Name).Get(context.TODO(), name, metav1.GetOptions{})
	}
	return rb, err
}

// delete a RoleBinding in the given Namespace.
func (nm *tenantNamespaceManager) unbindClusterRole(namespace *v1.Namespace, clusterRole string) error {
	name := getRoleBindingName(clusterRole)
	return client.IgnoreNotFound(nm.client.RbacV1().RoleBindings(namespace.Name).Delete(context.TODO(), name, metav1.DeleteOptions{}))
}

func getRoleBindingName(clusterRoleName string) string {
	return strings.Join([]string{roleBindingRoot, clusterRoleName}, "-")
}
