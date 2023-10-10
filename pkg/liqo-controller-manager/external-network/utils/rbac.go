// Copyright 2019-2023 The Liqo Authors
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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

// EnsureServiceAccountAndRoleBinding ensures that the service account and the role binding are created or deleted.
func EnsureServiceAccountAndRoleBinding(ctx context.Context, cl client.Client, s *runtime.Scheme,
	deploy *networkingv1alpha1.DeploymentTemplate, owner metav1.Object, clusterRoleName string) error {
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
	if _, err := controllerutil.CreateOrUpdate(ctx, cl, sa, func() error {
		return controllerutil.SetControllerReference(owner, sa, s)
	}); err != nil {
		klog.Errorf("error while creating service account %q: %v", saName, err)
		return err
	}

	// ensure role binding
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, cl, rb, func() error {
		rb.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		}
		rb.Subjects = []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      saName,
			Namespace: namespace,
		}}
		return controllerutil.SetControllerReference(owner, rb, s)
	}); err != nil {
		klog.Errorf("error while creating role binding %q: %v", name, err)
		return err
	}

	return nil
}
