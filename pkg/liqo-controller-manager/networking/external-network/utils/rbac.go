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

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// DeleteClusterRoleBinding deletes the cluster role bindings owned by the given object.
func DeleteClusterRoleBinding(ctx context.Context, cl client.Client, obj client.Object) error {
	var crbList rbacv1.ClusterRoleBindingList
	if err := cl.List(ctx, &crbList, client.MatchingLabels{
		consts.GatewayNameLabel:      obj.GetName(),
		consts.GatewayNamespaceLabel: obj.GetNamespace(),
	}); err != nil {
		klog.Errorf("error while listing cluster role bindings: %v", err)
		return err
	}

	for i := range crbList.Items {
		crb := &crbList.Items[i]
		if err := client.IgnoreNotFound(cl.Delete(ctx, crb)); err != nil {
			klog.Errorf("error while deleting cluster role binding %q: %v", crb.Name, err)
			return err
		}
	}
	return nil
}

// EnsureServiceAccountAndClusterRoleBinding ensures that the service account and the cluster role binding are created or deleted.
func EnsureServiceAccountAndClusterRoleBinding(ctx context.Context, cl client.Client, s *runtime.Scheme,
	deploy *networkingv1beta1.DeploymentTemplate, owner client.Object, clusterRoleName string) error {
	namespace := owner.GetNamespace()
	name := owner.GetName()

	saName := deploy.Spec.Template.Spec.ServiceAccountName
	if saName == "" {
		saName = "default"
	}

	// ensure service account
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
		},
	}
	if _, err := resource.CreateOrUpdate(ctx, cl, sa, func() error {
		return controllerutil.SetControllerReference(owner, sa, s)
	}); err != nil {
		klog.Errorf("error while creating service account %q: %v", saName, err)
		return err
	}

	// ensure cluster role binding
	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s-%s", clusterRoleName, namespace, name),
		},
	}
	if _, err := resource.CreateOrUpdate(ctx, cl, crb, func() error {
		if crb.Labels == nil {
			crb.Labels = make(map[string]string)
		}
		crb.Labels[consts.GatewayNameLabel] = name
		crb.Labels[consts.GatewayNamespaceLabel] = namespace
		crb.Labels[consts.K8sAppManagedByKey] = consts.LiqoAppLabelValue

		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		}
		crb.Subjects = []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      saName,
			Namespace: namespace,
		}}
		return nil
	}); err != nil {
		klog.Errorf("error while creating cluster role binding %q: %v", name, err)
		return err
	}

	controllerutil.AddFinalizer(owner, consts.ClusterRoleBindingFinalizer)

	return nil
}
