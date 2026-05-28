// Copyright 2019-2026 The Liqo Authors
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
		if controllerutil.ContainsFinalizer(crb, consts.ClusterRoleBindingFinalizer) {
			patch := client.MergeFrom(crb.DeepCopy())
			controllerutil.RemoveFinalizer(crb, consts.ClusterRoleBindingFinalizer)
			if err := cl.Patch(ctx, crb, patch); err != nil && !apierrors.IsNotFound(err) {
				klog.Errorf("error while removing finalizer from cluster role binding %q: %v", crb.Name, err)
				return err
			}
		}
		if err := client.IgnoreNotFound(cl.Delete(ctx, crb)); err != nil {
			klog.Errorf("error while deleting cluster role binding %q: %v", crb.Name, err)
			return err
		}
	}
	return nil
}

// CleanupClusterRoleBindings handles the full cleanup sequence for orphaned ClusterRoleBindings
// after the owning WgGateway resource has been deleted. It waits for gateway pods to terminate,
// removes the ServiceAccount finalizer, and then removes the CRB finalizer and deletes the CRB.
// Returns (requeue, error).
func CleanupClusterRoleBindings(ctx context.Context, cl client.Client, name, namespace string) (bool, error) {
	var crbList rbacv1.ClusterRoleBindingList
	if err := cl.List(ctx, &crbList, client.MatchingLabels{
		consts.GatewayNameLabel:      name,
		consts.GatewayNamespaceLabel: namespace,
	}); err != nil {
		return false, fmt.Errorf("listing cluster role bindings for gateway %s/%s: %w", namespace, name, err)
	}

	// Filter to only CRBs that still carry our finalizer.
	var pending []rbacv1.ClusterRoleBinding
	for i := range crbList.Items {
		if controllerutil.ContainsFinalizer(&crbList.Items[i], consts.ClusterRoleBindingFinalizer) {
			pending = append(pending, crbList.Items[i])
		}
	}
	if len(pending) == 0 {
		return false, nil
	}

	// Wait for all gateway pods to terminate before revoking RBAC.
	var podList corev1.PodList
	if err := cl.List(ctx, &podList,
		client.InNamespace(namespace),
		client.MatchingLabels{
			consts.GatewayNameLabel:      name,
			consts.GatewayNamespaceLabel: namespace,
		}); err != nil {
		return false, fmt.Errorf("listing gateway pods for %s/%s: %w", namespace, name, err)
	}
	if len(podList.Items) > 0 {
		klog.V(4).Infof("Waiting for %d gateway pod(s) to terminate before removing ClusterRoleBinding for %s/%s",
			len(podList.Items), namespace, name)
		return true, nil
	}

	// All pods are gone. Remove the ServiceAccount finalizer so the GC can clean it up.
	// Derive the SA name from the CRB subjects.
	for i := range pending {
		for _, subj := range pending[i].Subjects {
			if subj.Kind == rbacv1.ServiceAccountKind && subj.Namespace == namespace {
				var sa corev1.ServiceAccount
				if err := cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: subj.Name}, &sa); err != nil {
					if !apierrors.IsNotFound(err) {
						return false, fmt.Errorf("getting gateway service account %q: %w", subj.Name, err)
					}
				} else if controllerutil.ContainsFinalizer(&sa, consts.GatewayServiceAccountFinalizer) {
					patch := client.MergeFrom(sa.DeepCopy())
					controllerutil.RemoveFinalizer(&sa, consts.GatewayServiceAccountFinalizer)
					if err := cl.Patch(ctx, &sa, patch); err != nil && !apierrors.IsNotFound(err) {
						return false, fmt.Errorf("removing finalizer from gateway service account %q: %w", subj.Name, err)
					}
				}
			}
		}

		// Remove the finalizer from the CRB and delete it.
		patch := client.MergeFrom(pending[i].DeepCopy())
		controllerutil.RemoveFinalizer(&pending[i], consts.ClusterRoleBindingFinalizer)
		if err := cl.Patch(ctx, &pending[i], patch); err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("removing finalizer from cluster role binding %q: %w", pending[i].Name, err)
		}
		if err := client.IgnoreNotFound(cl.Delete(ctx, &pending[i])); err != nil {
			return false, fmt.Errorf("deleting cluster role binding %q: %w", pending[i].Name, err)
		}
	}

	return false, nil
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
		// Keep this finalizer so the GC cascade from owner deletion does not invalidate the
		// pod's service account token while the pod is still running shutdown cleanup.
		// The finalizer is removed explicitly after all pods have terminated.
		controllerutil.AddFinalizer(sa, consts.GatewayServiceAccountFinalizer)
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

		controllerutil.AddFinalizer(crb, consts.ClusterRoleBindingFinalizer)
		return nil
	}); err != nil {
		klog.Errorf("error while creating cluster role binding %q: %v", name, err)
		return err
	}

	return nil
}
