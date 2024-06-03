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

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/vkMachinery"
)

// VirtualKubeletName returns the name of the virtual-kubelet.
func VirtualKubeletName(virtualNode *virtualkubeletv1alpha1.VirtualNode) string {
	return "vk-" + virtualNode.Name
}

// VirtualKubeletDeployment forges the deployment for a virtual-kubelet.
func VirtualKubeletDeployment(homeCluster discoveryv1alpha1.ClusterID, virtualNode *virtualkubeletv1alpha1.VirtualNode,
	opts *VirtualKubeletOpts) *appsv1.Deployment {
	vkLabels := VirtualKubeletLabels(virtualNode, opts.ExtraLabels)
	annotations := opts.ExtraAnnotations
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        VirtualKubeletName(virtualNode),
			Namespace:   virtualNode.Namespace,
			Labels:      vkLabels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: vkLabels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      vkLabels,
					Annotations: annotations,
				},
				Spec: forgeVKPodSpec(virtualNode.Namespace, homeCluster, virtualNode, opts),
			},
		},
	}
}

// VirtualKubeletLabels forges the labels for a virtual-kubelet.
func VirtualKubeletLabels(virtualNode *virtualkubeletv1alpha1.VirtualNode, extraLabels map[string]string) map[string]string {
	return labels.Merge(labels.Merge(extraLabels, vkMachinery.KubeletBaseLabels), map[string]string{
		discovery.ClusterIDLabel:   string(virtualNode.Spec.ClusterID),
		discovery.VirtualNodeLabel: virtualNode.Name,
	})
}

// ClusterRoleLabels returns the labels to be set on a ClusterRoleBinding related to a VirtualKubelet.
func ClusterRoleLabels(remoteClusterID discoveryv1alpha1.ClusterID) map[string]string {
	return labels.Merge(vkMachinery.ClusterRoleBindingLabels, map[string]string{
		discovery.ClusterIDLabel: string(remoteClusterID),
	})
}

// VirtualKubeletClusterRoleBinding forges a ClusterRoleBinding for a VirtualKubelet.
func VirtualKubeletClusterRoleBinding(kubeletNamespace, kubeletName string,
	remoteCluster discoveryv1alpha1.ClusterID) *rbacv1.ClusterRoleBinding {
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
