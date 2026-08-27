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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// DeleteAllForeignClusters deletes all ForeignCluster resources.
func DeleteAllForeignClusters(ctx context.Context, client dynamic.Interface) error {
	err := deleteAllResources(ctx, client, liqov1beta1.ForeignClusterGroupVersionResource)
	if err != nil {
		return fmt.Errorf("deleting ForeignCluster resources: %w", err)
	}

	return nil
}

// DeleteInternalNodes deletes all InternalNode resources.
func DeleteInternalNodes(ctx context.Context, client dynamic.Interface) error {
	err := deleteAllResources(ctx, client, networkingv1beta1.InternalNodeGroupVersionResource)
	if err != nil {
		return fmt.Errorf("deleting InternalNode resources: %w", err)
	}

	return nil
}

// DeleteNetworks deletes the Networks installed.
func DeleteNetworks(ctx context.Context, client dynamic.Interface) error {
	err := deleteAllResources(ctx, client, ipamv1alpha1.NetworkGroupVersionResource)
	if err != nil {
		return fmt.Errorf("deleting Network resources: %w", err)
	}

	return nil
}

// DeleteIPs deletes the IPs installed.
func DeleteIPs(ctx context.Context, client dynamic.Interface) error {
	err := deleteAllResources(ctx, client, ipamv1alpha1.IPGroupVersionResource)
	if err != nil {
		return fmt.Errorf("deleting IP resources: %w", err)
	}

	return nil
}

// DeleteNamespaceOffloadings deletes all NamespaceOffloading resources.
func DeleteNamespaceOffloadings(ctx context.Context, client dynamic.Interface) error {
	err := deleteAllResources(ctx, client, offloadingv1beta1.NamespaceOffloadingGroupVersionResource)
	if err != nil {
		return fmt.Errorf("deleting NamespaceOffloading resources: %w", err)
	}

	return nil
}

// DeleteVirtualNodes deletes all VirtualNode resources.
func DeleteVirtualNodes(ctx context.Context, client dynamic.Interface) error {
	err := deleteAllResources(ctx, client, offloadingv1beta1.VirtualNodeGroupVersionResource)
	if err != nil {
		return fmt.Errorf("deleting VirtualNode resources: %w", err)
	}

	return nil
}

// DeleteResourceSlices deletes all ResourceSlice resources.
func DeleteResourceSlices(ctx context.Context, client dynamic.Interface) error {
	err := deleteAllResources(ctx, client, authv1beta1.ResourceSliceGroupVersionResource)
	if err != nil {
		return fmt.Errorf("deleting ResourceSlice resources: %w", err)
	}

	return nil
}

// DeleteTenantNamespaces deletes all namespaces labeled as Liqo tenant namespaces.
func DeleteTenantNamespaces(ctx context.Context, client dynamic.Interface) error {
	r1 := client.Resource(corev1.SchemeGroupVersion.WithResource("namespaces"))
	selector := metav1.FormatLabelSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{consts.TenantNamespaceLabel: "true"},
	})
	unstructured, err := r1.List(ctx, metav1.ListOptions{LabelSelector: selector})
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

func deleteAllResources(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource) error {
	res := client.Resource(gvr)
	unstructured, err := res.List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("getting %v resources: %w", gvr, err)
	}

	for _, item := range unstructured.Items {
		err := res.Namespace(item.GetNamespace()).Delete(ctx, item.GetName(), metav1.DeleteOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("deleting %v resource %v: %w", gvr, item.GetName(), err)
		}
	}

	return nil
}
