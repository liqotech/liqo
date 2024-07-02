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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/strings"

	liqov1alpha1 "github.com/liqotech/liqo/apis/core/v1alpha1"
	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/vkMachinery"
)

// VirtualKubeletName returns the name of the virtual-kubelet.
func VirtualKubeletName(virtualNode *vkv1alpha1.VirtualNode) string {
	return "vk-" + virtualNode.Name
}

// VirtualKubeletDeployment forges the deployment for a virtual-kubelet.
func VirtualKubeletDeployment(homeCluster liqov1alpha1.ClusterID, localPodCIDR, liqoNamespace string,
	virtualNode *vkv1alpha1.VirtualNode, opts *vkv1alpha1.VkOptionsTemplate) *appsv1.Deployment {
	matchLabels := VirtualKubeletLabels(virtualNode) // these are the minimum set of labels used as selector
	depLabels := labels.Merge(opts.Spec.ExtraLabels, matchLabels)
	depAnnotations := opts.Spec.ExtraAnnotations
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        VirtualKubeletName(virtualNode),
			Namespace:   virtualNode.Namespace,
			Labels:      depLabels,
			Annotations: depAnnotations,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      depLabels,
					Annotations: depAnnotations,
				},
				Spec: forgeVKPodSpec(virtualNode.Namespace, homeCluster, localPodCIDR, liqoNamespace, virtualNode, opts),
			},
		},
	}
}

// VirtualKubeletLabels forges the labels for a virtual-kubelet.
func VirtualKubeletLabels(virtualNode *vkv1alpha1.VirtualNode) map[string]string {
	return labels.Merge(vkMachinery.KubeletBaseLabels, map[string]string{
		consts.RemoteClusterID:  string(virtualNode.Spec.ClusterID),
		consts.VirtualNodeLabel: virtualNode.Name,
	})
}

// ClusterRoleLabels returns the labels to be set on a ClusterRoleBinding related to a VirtualKubelet.
func ClusterRoleLabels(remoteClusterID liqov1alpha1.ClusterID) map[string]string {
	return labels.Merge(vkMachinery.ClusterRoleBindingLabels, map[string]string{
		consts.RemoteClusterID: string(remoteClusterID),
	})
}

// VirtualKubeletClusterRoleBinding forges a ClusterRoleBinding for a VirtualKubelet.
func VirtualKubeletClusterRoleBinding(kubeletNamespace, kubeletName string,
	remoteCluster liqov1alpha1.ClusterID) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   strings.ShortenString(fmt.Sprintf("%s%s", vkMachinery.CRBPrefix, kubeletName), 253),
			Labels: ClusterRoleLabels(remoteCluster),
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", APIGroup: "", Name: kubeletName, Namespace: kubeletNamespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     vkMachinery.LocalClusterRoleName,
		},
	}
}

// VirtualKubeletServiceAccount forges a ServiceAccount for a VirtualKubelet.
func VirtualKubeletServiceAccount(namespace, name string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
