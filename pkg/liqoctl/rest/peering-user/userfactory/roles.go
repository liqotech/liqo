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
//

package userfactory

import (
	"context"
	"fmt"

	certv1 "k8s.io/api/certificates/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

var peeringUserLabel = client.ListOptions{
	LabelSelector: labels.SelectorFromSet(labels.Set{
		"app.kubernetes.io/component": "peering-user",
	}),
}

var minimumClusterPermissions = []rbacv1.PolicyRule{
	{
		APIGroups: []string{""},
		Resources: []string{"namespaces"},
		Verbs:     []string{"get", "list", "create"},
	},
	{
		APIGroups: []string{ipamv1alpha1.NetworkGroupResource.Group},
		Resources: []string{ipamv1alpha1.NetworkResource},
		Verbs:     []string{"get", "list"},
	},
	{
		APIGroups: []string{networkingv1beta1.ConfigurationGroupResource.Group},
		Resources: []string{networkingv1beta1.ConfigurationResource},
		Verbs:     []string{"get", "list"},
	},
	{
		APIGroups: []string{networkingv1beta1.GatewayClientGroupResource.Group},
		Resources: []string{networkingv1beta1.GatewayClientResource},
		Verbs:     []string{"get", "list"},
	},
	{
		APIGroups: []string{networkingv1beta1.GatewayServerGroupResource.Group},
		Resources: []string{networkingv1beta1.GatewayServerResource},
		Verbs:     []string{"get", "list"},
	},
	{
		APIGroups: []string{networkingv1beta1.PublicKeyGroupResource.Group},
		Resources: []string{networkingv1beta1.PublicKeyResource},
		Verbs:     []string{"get", "list"},
	},
	{
		APIGroups: []string{liqov1beta1.ForeignClusterGroupResource.Group},
		Resources: []string{liqov1beta1.ForeignClusterResource},
		Verbs:     []string{"get", "list"},
	},
	{
		APIGroups: []string{authv1beta1.TenantGroupResource.Group},
		Resources: []string{authv1beta1.TenantResource},
		Verbs:     []string{"create", "list"},
	},
}

// EnsureRoles ensures that the required roles are created and bound to the user.
func EnsureRoles(ctx context.Context, c client.Client, clusterID liqov1beta1.ClusterID, userCN, tenantNsName string) error {
	if err := ensureLiqoNsReaderRole(ctx, c, userCN, clusterID); err != nil {
		return err
	}

	if err := ensureTenantNsWriterRole(ctx, c, userCN, clusterID, tenantNsName); err != nil {
		return err
	}

	if err := ensureClusterMinPermissions(ctx, c, userCN, clusterID); err != nil {
		return err
	}

	return nil
}

// IsExistingPeerUser checks whether the user has already been created.
func IsExistingPeerUser(ctx context.Context, c client.Client, clusterID liqov1beta1.ClusterID) (bool, error) {
	userName := GetUserNameFromClusterID(clusterID)

	clusterRoleList := &rbacv1.ClusterRoleList{}
	if err := c.List(ctx, clusterRoleList, &client.ListOptions{
		LabelSelector: getUserLabelSelector(userName),
	}); err != nil {
		return false, fmt.Errorf("unable to check whether the user has already been created: %w", err)
	}

	return len(clusterRoleList.Items) > 0, nil
}

// RemovePermissions removes the permissions related to the user.
func RemovePermissions(ctx context.Context, c client.Client, clusterID liqov1beta1.ClusterID) error {
	userName := GetUserNameFromClusterID(clusterID)

	userLabelSelector := getUserLabelSelector(userName)

	// Delete the ClusterRole related to the user
	if err := c.DeleteAllOf(ctx, &rbacv1.ClusterRole{}, client.MatchingLabelsSelector{Selector: userLabelSelector}); err != nil {
		return fmt.Errorf("unable to delete ClusterRoles: %w", err)
	}

	// Delete the ClusterRoleBindings related to the user
	if err := c.DeleteAllOf(ctx, &rbacv1.ClusterRoleBinding{}, client.MatchingLabelsSelector{Selector: userLabelSelector}); err != nil {
		return fmt.Errorf("unable to delete ClusterRoleBindings: %w", err)
	}

	// Cannot delete RoleBinding with DeleteAllOf, list it and delete one by one
	roleBindingList := &rbacv1.RoleBindingList{}

	if err := c.List(ctx, roleBindingList, &client.ListOptions{
		LabelSelector: userLabelSelector,
	}); err != nil {
		return fmt.Errorf("unable to get RoleBindings: %w", err)
	}

	for i := range roleBindingList.Items {
		err := c.Delete(ctx, &roleBindingList.Items[i])
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("unable to delete RoleBinding %q: %w", roleBindingList.Items[i].Name, err)
		}
	}

	// Delete the CertificateSigningRequest with the certificate of the user
	if err := c.DeleteAllOf(
		ctx,
		&certv1.CertificateSigningRequest{},
		client.MatchingLabelsSelector{Selector: userLabelSelector},
	); err != nil {
		return fmt.Errorf("unable to delete ClusterRoleBindings: %w", err)
	}

	return nil
}

func getUserLabelSelector(userName string) labels.Selector {
	return labels.SelectorFromSet(labels.Set{
		consts.PeeringUserNameLabelKey: userName,
	})
}

// ensureLiqoNsReaderRole ensures that the peering-user Role is bound to the user in the Liqo namespace.
func ensureLiqoNsReaderRole(ctx context.Context, c client.Client, userCN string, clusterID liqov1beta1.ClusterID) error {
	var peeringUserRoleList rbacv1.RoleList
	if err := c.List(ctx, &peeringUserRoleList, &peeringUserLabel); err != nil {
		return fmt.Errorf("unable to get peering-user Role from liqo namespace: %w", err)
	}

	if nRoles := len(peeringUserRoleList.Items); nRoles == 0 {
		return fmt.Errorf("no peering-user Role found in the Liqo namespace")
	} else if nRoles > 1 {
		return fmt.Errorf("multiple peering-user Roles found in the Liqo namespace")
	}

	peeringUserRole := peeringUserRoleList.Items[0]
	userName := GetUserNameFromClusterID(clusterID)

	// Bind the roles to operate on the liqo namespace
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-liqo-ns-reader", userName),
			Namespace: peeringUserRole.Namespace,
			Labels: map[string]string{
				consts.PeeringUserNameLabelKey: userName,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     userCN,
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     peeringUserRole.Name,
		},
	}

	if err := c.Create(ctx, roleBinding); err != nil {
		return fmt.Errorf("unable to create role binding in the %q namespace: %w", peeringUserRole.Namespace, err)
	}

	return nil
}

func ensureTenantNsWriterRole(ctx context.Context, c client.Client, userCN string, clusterID liqov1beta1.ClusterID, tenantNsName string) error {
	var peeringClusterRoles rbacv1.ClusterRoleList
	if err := c.List(ctx, &peeringClusterRoles, &peeringUserLabel); err != nil {
		return fmt.Errorf("unable to get peering-user role from liqo namespace: %w", err)
	}

	if nRoles := len(peeringClusterRoles.Items); nRoles == 0 {
		return fmt.Errorf("no peering-user ClusterRole found")
	} else if nRoles > 1 {
		return fmt.Errorf("multiple peering-user ClusterRoles found ")
	}

	// bind the ClusterRole to the userName user
	userName := GetUserNameFromClusterID(clusterID)
	peeringUserClusterRole := peeringClusterRoles.Items[0]
	clusterRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-tenant-ns-writer", userName),
			Namespace: tenantNsName,
			Labels: map[string]string{
				consts.PeeringUserNameLabelKey: userName,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     userCN,
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     peeringUserClusterRole.Name,
		},
	}

	if err := c.Create(ctx, clusterRoleBinding); err != nil {
		return fmt.Errorf("unable to create cluster role binding: %w", err)
	}

	return nil
}

func ensureClusterMinPermissions(ctx context.Context, c client.Client, userCN string, clusterID liqov1beta1.ClusterID) error {
	userName := GetUserNameFromClusterID(clusterID)

	// Append to the minimum permissions the permissions to operate on the user Tenant resource
	permissions := append(
		[]rbacv1.PolicyRule{
			{
				APIGroups:     []string{authv1beta1.TenantGroupResource.Group},
				Resources:     []string{authv1beta1.TenantResource},
				ResourceNames: []string{string(clusterID)},
				Verbs:         []string{"update", "get", "delete"},
			},
		},
		minimumClusterPermissions...,
	)

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-cluster-min-perm", userName),
			Labels: map[string]string{
				consts.PeeringUserNameLabelKey: userName,
			},
		},
		Rules: permissions,
	}

	if err := c.Create(ctx, clusterRole); err != nil {
		return fmt.Errorf("unable to create ClusterRole for minimum permissions on the cluster: %w", err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-cluster-min-perm", userName),
			Labels: map[string]string{
				consts.PeeringUserNameLabelKey: userName,
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     userCN,
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
	}

	if err := c.Create(ctx, clusterRoleBinding); err != nil {
		return fmt.Errorf("unable to create ClusterRoleBinding for minimum permissions on the cluster: %w", err)
	}

	return nil
}
