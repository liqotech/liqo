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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructuredpkg "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
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

// firewallConfigurationBindingControllerFinalizer is the finalizer set by the
// firewall controller on FirewallConfigurationBinding resources.
const firewallConfigurationBindingControllerFinalizer = "firewallconfigurationbinding-controller.liqo.io/finalizer"

// DeleteFirewallConfigurationBindings deletes all FirewallConfigurationBinding resources,
// first removing their controller finalizer so they are not stuck during uninstall.
func DeleteFirewallConfigurationBindings(ctx context.Context, client dynamic.Interface) error {
	r1 := client.Resource(networkingv1beta1.FirewallConfigurationBindingGroupVersionResource)
	ul, err := r1.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for i := range ul.Items {
		item := &ul.Items[i]
		// Remove the controller finalizer so the binding can be deleted even if
		// the fabric/gateway pod that owns it is already gone.
		finalizers, found, err := unstructuredpkg.NestedStringSlice(item.Object, "metadata", "finalizers")
		if err != nil {
			return err
		}
		if found {
			newFinalizers := make([]string, 0, len(finalizers))
			for _, f := range finalizers {
				if f != firewallConfigurationBindingControllerFinalizer {
					newFinalizers = append(newFinalizers, f)
				}
			}
			if err := unstructuredpkg.SetNestedStringSlice(item.Object, newFinalizers, "metadata", "finalizers"); err != nil {
				return err
			}
			if _, err := r1.Namespace(item.GetNamespace()).Update(ctx, item, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}

		if err := r1.Namespace(item.GetNamespace()).Delete(ctx, item.GetName(), metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}
