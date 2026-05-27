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

package uninstaller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// DeleteAllForeignClusters deletes all ForeignCluster resources.
func DeleteAllForeignClusters(ctx context.Context, client dynamic.Interface) error {
	r1 := client.Resource(liqov1beta1.ForeignClusterGroupVersionResource)
	err := r1.DeleteCollection(ctx,
		metav1.DeleteOptions{TypeMeta: metav1.TypeMeta{}}, metav1.ListOptions{TypeMeta: metav1.TypeMeta{}})
	return err
}

// DeleteInternalNodes deletes all InternalNode resources.
func DeleteInternalNodes(ctx context.Context, client dynamic.Interface) error {
	r1 := client.Resource(networkingv1beta1.InternalNodeGroupVersionResource)
	unstructured, err := r1.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, item := range unstructured.Items {
		if err := r1.Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// DeleteNetworks deletes the Networks installed.
func DeleteNetworks(ctx context.Context, client dynamic.Interface) error {
	r1 := client.Resource(ipamv1alpha1.NetworkGroupVersionResource)
	unstructured, err := r1.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, item := range unstructured.Items {
		if err := r1.Namespace(item.GetNamespace()).Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// DeleteIPs deletes the IPs installed.
func DeleteIPs(ctx context.Context, client dynamic.Interface) error {
	r1 := client.Resource(ipamv1alpha1.IPGroupVersionResource)
	unstructured, err := r1.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, item := range unstructured.Items {
		if err := r1.Namespace(item.GetNamespace()).Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// DeleteHelmKeepResources deletes RBAC resources managed by Liqo that are annotated with
// "helm.sh/resource-policy: keep". These resources are retained during helm uninstall
// to allow the fabric DaemonSet pods to clean up their finalizers, and must be explicitly
// deleted once the cleanup is complete.
func DeleteHelmKeepResources(ctx context.Context, namespace string, client dynamic.Interface) error {
	// Delete ClusterRoleBindings with the keep annotation managed by Liqo.
	crbGVR := rbacv1.SchemeGroupVersion.WithResource("clusterrolebindings")
	crbList, err := client.Resource(crbGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing ClusterRoleBindings: %w", err)
	}
	for i := range crbList.Items {
		item := &crbList.Items[i]
		if item.GetAnnotations()[consts.HelmResourcePolicyAnnotationKey] == consts.HelmResourcePolicyAnnotationKeepValue &&
			item.GetLabels()[consts.K8sAppPartOfKey] == consts.K8sAppPartOfLiqoValue {
			klog.Infof("Deleting ClusterRoleBinding %s (%s: %s)", item.GetName(),
				consts.HelmResourcePolicyAnnotationKey, consts.HelmResourcePolicyAnnotationKeepValue)
			if err := client.Resource(crbGVR).Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("deleting ClusterRoleBinding %s: %w", item.GetName(), err)
			}
		}
	}

	// Delete ClusterRoles with the keep annotation managed by Liqo.
	crGVR := rbacv1.SchemeGroupVersion.WithResource("clusterroles")
	crList, err := client.Resource(crGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing ClusterRoles: %w", err)
	}
	for i := range crList.Items {
		item := &crList.Items[i]
		if item.GetAnnotations()[consts.HelmResourcePolicyAnnotationKey] == consts.HelmResourcePolicyAnnotationKeepValue &&
			item.GetLabels()[consts.K8sAppPartOfKey] == consts.K8sAppPartOfLiqoValue {
			klog.Infof("Deleting ClusterRole %s (%s: %s)", item.GetName(),
				consts.HelmResourcePolicyAnnotationKey, consts.HelmResourcePolicyAnnotationKeepValue)
			if err := client.Resource(crGVR).Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("deleting ClusterRole %s: %w", item.GetName(), err)
			}
		}
	}

	// Delete ServiceAccounts with the keep annotation managed by Liqo.
	saGVR := corev1.SchemeGroupVersion.WithResource("serviceaccounts")
	saList, err := client.Resource(saGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing ServiceAccounts: %w", err)
	}
	for i := range saList.Items {
		item := &saList.Items[i]
		if item.GetAnnotations()[consts.HelmResourcePolicyAnnotationKey] == consts.HelmResourcePolicyAnnotationKeepValue &&
			item.GetLabels()[consts.K8sAppPartOfKey] == consts.K8sAppPartOfLiqoValue {
			klog.Infof("Deleting ServiceAccount %s/%s (%s: %s)", item.GetNamespace(), item.GetName(),
				consts.HelmResourcePolicyAnnotationKey, consts.HelmResourcePolicyAnnotationKeepValue)
			if err := client.Resource(saGVR).Namespace(namespace).Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil {
				return fmt.Errorf("deleting ServiceAccount %s/%s: %w", item.GetNamespace(), item.GetName(), err)
			}
		}
	}

	return nil
}
