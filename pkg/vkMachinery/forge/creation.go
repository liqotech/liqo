// Copyright 2019-2022 The Liqo Authors
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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/vkMachinery"
)

// VirtualKubeletDeployment forges the deployment for a virtual-kubelet.
func VirtualKubeletDeployment(homeCluster, remoteCluster discoveryv1alpha1.ClusterIdentity, vkName, vkNamespace, liqoNamespace,
	nodeName string, opts *VirtualKubeletOpts, resourceOffer *sharingv1alpha1.ResourceOffer) (*appsv1.Deployment, error) {
	vkLabels := VirtualKubeletLabels(remoteCluster.ClusterID, opts)
	annotations := opts.ExtraAnnotations
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        vkName,
			Namespace:   vkNamespace,
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
				Spec: forgeVKPodSpec(vkName, vkNamespace, liqoNamespace, homeCluster, remoteCluster, nodeName, opts, resourceOffer),
			},
		},
	}, nil
}

// VirtualKubeletLabels forges the labels for a virtual-kubelet.
func VirtualKubeletLabels(remoteClusterID string, opts *VirtualKubeletOpts) map[string]string {
	kubeletDynamicLabels := map[string]string{
		discovery.ClusterIDLabel: remoteClusterID,
	}
	return merge(vkMachinery.KubeletBaseLabels, kubeletDynamicLabels, opts.ExtraLabels)
}

func merge(m map[string]string, ms ...map[string]string) map[string]string {
	for _, s := range ms {
		for k, v := range s {
			m[k] = v
		}
	}
	return m
}

// ClusterRoleLabels returns the labels to be set on a ClusterRoleBinding related to a VirtualKubelet.
func ClusterRoleLabels(remoteClusterID string) map[string]string {
	dynamicLabels := map[string]string{
		discovery.ClusterIDLabel: remoteClusterID,
	}
	return merge(vkMachinery.ClusterRoleBindingLabels, dynamicLabels)
}

// VirtualKubeletClusterRoleBinding forges a ClusterRoleBinding for a VirtualKubelet.
func VirtualKubeletClusterRoleBinding(name, kubeletNamespace, remoteClusterID string) *rbacv1.ClusterRoleBinding {
	labels := ClusterRoleLabels(remoteClusterID)
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", APIGroup: "", Name: name, Namespace: kubeletNamespace},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     vkMachinery.VKClusterRoleName,
		},
	}
}

// VirtualKubeletServiceAccount forges a ServiceAccount for a VirtualKubelet.
func VirtualKubeletServiceAccount(name, kubeletNamespace string) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: kubeletNamespace,
		},
	}
}
