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
	"context"
	"fmt"

	capsulev1beta1 "github.com/clastix/capsule/api/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/discovery"
)

const (
	clusterRolePrefix = "liqo-remote-cluster-role"
	tenantPrefix      = "tenant"
)

// BindOutgoingClusterWideRole creates and binds a ClusterRole for the cluster-wide permission required
// to establish the peering by the remote cluster.
func (nm *tenantNamespaceManager) BindOutgoingClusterWideRole(ctx context.Context,
	clusterID string) (*rbacv1.ClusterRoleBinding, error) {
	clusterRoleName := getClusterRoleName(clusterID)
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				discovery.ClusterIDLabel: clusterID,
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{capsulev1beta1.GroupVersion.Group},
				Resources:     []string{"tenants/finalizers"},
				Verbs:         []string{"get", "patch", "update"},
				ResourceNames: []string{getTenantName(clusterID)},
			},
		},
	}

	_, err := nm.client.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		klog.Error(err)
		return nil, err
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				discovery.ClusterIDLabel: clusterID,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     rbacv1.UserKind,
				APIGroup: rbacv1.GroupName,
				Name:     clusterID,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
	}

	clusterRoleBinding, err = nm.client.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		klog.Error(err)
		return nil, err
	}

	return clusterRoleBinding, nil
}

// UnbindOutgoingClusterWideRole unbinds and deletes a ClusterRole for the cluster-wide permission required
// to establish the peering by the remote cluster.
func (nm *tenantNamespaceManager) UnbindOutgoingClusterWideRole(ctx context.Context, clusterID string) error {
	clusterRoleName := getClusterRoleName(clusterID)

	err := nm.client.RbacV1().ClusterRoleBindings().Delete(ctx, clusterRoleName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Error(err)
		return err
	}

	err = nm.client.RbacV1().ClusterRoles().Delete(ctx, clusterRoleName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Error(err)
		return err
	}

	return nil
}

func getClusterRoleName(clusterID string) string {
	return fmt.Sprintf("%v-%v", clusterRolePrefix, clusterID)
}

func getTenantName(clusterID string) string {
	return fmt.Sprintf("%v-%v", tenantPrefix, clusterID)
}
