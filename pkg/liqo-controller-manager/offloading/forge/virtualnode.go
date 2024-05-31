// Copyright 2019-2024 The Liqo Authors
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

package forge

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
)

// VirtualNodeOptions contains the options to forge a VirtualNode resource.
type VirtualNodeOptions struct {
	KubeconfigSecretRef corev1.LocalObjectReference `json:"kubeconfigSecretRef,omitempty"`
	CreateNode          bool                        `json:"createNode,omitempty"`
	DisableNetworkCheck bool                        `json:"disableNetworkCheck,omitempty"`

	ResourceList        corev1.ResourceList             `json:"resourceList,omitempty"`
	StorageClasses      []authv1alpha1.StorageType      `json:"storageClasses,omitempty"`
	IngressClasses      []authv1alpha1.IngressType      `json:"ingressClasses,omitempty"`
	LoadBalancerClasses []authv1alpha1.LoadBalancerType `json:"loadBalancerClasses,omitempty"`
	NodeLabels          map[string]string               `json:"nodeLabels,omitempty"`
	NodeSelector        map[string]string               `json:"nodeSelector,omitempty"`
}

// VirtualNode forges a VirtualNode resource.
func VirtualNode(name, namespace string) *vkv1alpha1.VirtualNode {
	return &vkv1alpha1.VirtualNode{
		TypeMeta: metav1.TypeMeta{
			APIVersion: vkv1alpha1.VirtualNodeGroupVersionResource.GroupVersion().String(),
			Kind:       vkv1alpha1.VirtualNodeKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// MutateVirtualNode mutates a VirtualNode resource.
func MutateVirtualNode(virtualNode *vkv1alpha1.VirtualNode,
	remoteClusterID discoveryv1alpha1.ClusterID, opts *VirtualNodeOptions) error {
	// VirtualNode metadata
	if virtualNode.ObjectMeta.Labels == nil {
		virtualNode.ObjectMeta.Labels = make(map[string]string)
	}
	virtualNode.ObjectMeta.Labels[discovery.ClusterIDLabel] = string(remoteClusterID)
	virtualNode.ObjectMeta.Labels[consts.RemoteClusterID] = string(remoteClusterID)

	// VirtualNode spec

	// node labels
	if virtualNode.Spec.Labels == nil {
		virtualNode.Spec.Labels = make(map[string]string)
	}
	virtualNode.Spec.Labels[discovery.ClusterIDLabel] = string(remoteClusterID)
	virtualNode.Spec.Labels = labels.Merge(virtualNode.Spec.Labels, opts.NodeLabels)

	virtualNode.Spec.ClusterID = remoteClusterID
	virtualNode.Spec.CreateNode = &opts.CreateNode
	virtualNode.Spec.DisableNetworkCheck = opts.DisableNetworkCheck
	virtualNode.Spec.KubeconfigSecretRef = &opts.KubeconfigSecretRef
	virtualNode.Spec.ResourceQuota = corev1.ResourceQuotaSpec{
		Hard: opts.ResourceList,
	}
	virtualNode.Spec.StorageClasses = opts.StorageClasses
	virtualNode.Spec.IngressClasses = opts.IngressClasses
	virtualNode.Spec.LoadBalancerClasses = opts.LoadBalancerClasses

	if len(opts.NodeSelector) > 0 {
		if virtualNode.Spec.OffloadingPatch == nil {
			virtualNode.Spec.OffloadingPatch = &vkv1alpha1.OffloadingPatch{}
		}

		virtualNode.Spec.OffloadingPatch.NodeSelector = opts.NodeSelector
	}

	return nil
}

// VirtualNodeOptionsFromResourceSlice extracts the VirtualNodeOptions from a ResourceSlice.
func VirtualNodeOptionsFromResourceSlice(resourceSlice *authv1alpha1.ResourceSlice,
	kubeconfigSecretName string, createNode, disableNetworkCheck bool) *VirtualNodeOptions {
	return &VirtualNodeOptions{
		KubeconfigSecretRef: corev1.LocalObjectReference{Name: kubeconfigSecretName},
		CreateNode:          createNode,
		DisableNetworkCheck: disableNetworkCheck,

		ResourceList:        resourceSlice.Status.Resources,
		StorageClasses:      resourceSlice.Status.StorageClasses,
		IngressClasses:      resourceSlice.Status.IngressClasses,
		LoadBalancerClasses: resourceSlice.Status.LoadBalancerClasses,
		NodeLabels:          resourceSlice.Status.NodeLabels,
		NodeSelector:        resourceSlice.Status.NodeSelector,
	}
}
