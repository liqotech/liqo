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

package forge

import (
	"fmt"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"k8s.io/utils/strings"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/vkMachinery"
)

// vkNamePrefix is the prefix of the names of the virtual-kubelet resources (e.g., the
// Deployment and the ServiceAccount ones), to avoid collisions with other components'
// resources named after the cluster ID.
const vkNamePrefix = "vk-"

// VirtualKubeletName returns the name of the virtual-kubelet.
func VirtualKubeletName(virtualNode *offloadingv1beta1.VirtualNode) string {
	return vkNamePrefix + virtualNode.Name
}

// VirtualKubeletDeployment forges the deployment for a virtual-kubelet.
func VirtualKubeletDeployment(homeCluster liqov1beta1.ClusterID, liqoNamespace string, localPodCIDRs []string,
	virtualNode *offloadingv1beta1.VirtualNode, opts *offloadingv1beta1.VkOptionsTemplate) *appsv1.Deployment {
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
			Replicas: opts.Spec.Replicas,
			// The rolling update is surge-only: the old pod is not deleted before the new one
			// is ready, so that a virtual-kubelet failing to start (e.g., due to invalid
			// arguments) does not leave the node without a running virtual-kubelet.
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       ptr.To(intstr.FromInt32(1)),
					MaxUnavailable: ptr.To(intstr.FromInt32(0)),
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      depLabels,
					Annotations: depAnnotations,
				},
				Spec: forgeVKPodSpec(virtualNode.Namespace, homeCluster, liqoNamespace, localPodCIDRs, virtualNode, opts),
			},
		},
	}
}

// VirtualKubeletLabels forges the labels for a virtual-kubelet.
func VirtualKubeletLabels(virtualNode *offloadingv1beta1.VirtualNode) map[string]string {
	return labels.Merge(vkMachinery.KubeletBaseLabels, map[string]string{
		consts.RemoteClusterID:  string(virtualNode.Spec.ClusterID),
		consts.VirtualNodeLabel: virtualNode.Name,
	})
}

// ClusterRoleLabels returns the labels to be set on a ClusterRoleBinding related to a VirtualKubelet.
func ClusterRoleLabels(remoteClusterID liqov1beta1.ClusterID) map[string]string {
	return labels.Merge(vkMachinery.ClusterRoleBindingLabels, map[string]string{
		consts.RemoteClusterID: string(remoteClusterID),
	})
}

// VirtualKubeletClusterRoleBindingName returns the name of the ClusterRoleBinding of a VirtualKubelet.
func VirtualKubeletClusterRoleBindingName(virtualNodeName string) string {
	return strings.ShortenString(fmt.Sprintf("%s%s", vkMachinery.CRBPrefix, virtualNodeName), 253)
}

// VirtualKubeletClusterRoleBindingMutateFn returns a mutate function enforcing the desired
// state on the given ClusterRoleBinding.
func VirtualKubeletClusterRoleBindingMutateFn(crb *rbacv1.ClusterRoleBinding, virtualNode *offloadingv1beta1.VirtualNode) func() error {
	return func() error {
		crb.Labels = labels.Merge(crb.Labels, ClusterRoleLabels(virtualNode.Spec.ClusterID))
		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     vkMachinery.LocalClusterRoleName,
		}

		// The desired subject is added without removing the existing ones, so that legacy subjects (targeting the previous
		// ServiceAccount name) are preserved and virtual-kubelets from older versions keep their permissions during the rollout.
		sa := VirtualKubeletServiceAccount(virtualNode)
		desiredSubject := rbacv1.Subject{
			Kind:      "ServiceAccount", //nolint:goconst // only two occurrences, keeping the RBAC kind inline reads better
			APIGroup:  "",
			Name:      sa.Name,
			Namespace: sa.Namespace,
		}
		if !slices.Contains(crb.Subjects, desiredSubject) {
			crb.Subjects = append(crb.Subjects, desiredSubject)
		}

		return nil
	}
}

// VirtualKubeletAuthDelegatorClusterRoleBindingName returns the name of the auth-delegator
// ClusterRoleBinding of a VirtualKubelet.
func VirtualKubeletAuthDelegatorClusterRoleBindingName(virtualNodeName string) string {
	return strings.ShortenString(fmt.Sprintf("%s%s", vkMachinery.AuthDelegatorCRBPrefix, virtualNodeName), 253)
}

// VirtualKubeletAuthDelegatorClusterRoleBindingMutateFn returns a mutate function enforcing the
// desired state on the given auth-delegator ClusterRoleBinding.
func VirtualKubeletAuthDelegatorClusterRoleBindingMutateFn(crb *rbacv1.ClusterRoleBinding,
	virtualNode *offloadingv1beta1.VirtualNode) func() error {
	return func() error {
		crb.Labels = labels.Merge(crb.Labels, ClusterRoleLabels(virtualNode.Spec.ClusterID))
		crb.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:auth-delegator",
		}

		// The desired subject is added without removing the existing ones, so that legacy subjects (targeting the previous
		// ServiceAccount name) are preserved and virtual-kubelets from older versions keep their permissions during the rollout.
		sa := VirtualKubeletServiceAccount(virtualNode)
		desiredSubject := rbacv1.Subject{
			Kind:      "ServiceAccount", //nolint:goconst // only two occurrences, keeping the RBAC kind inline reads better
			APIGroup:  "",
			Name:      sa.Name,
			Namespace: sa.Namespace,
		}
		if !slices.Contains(crb.Subjects, desiredSubject) {
			crb.Subjects = append(crb.Subjects, desiredSubject)
		}

		return nil
	}
}

// VirtualKubeletServiceAccountName returns the name of the ServiceAccount of a VirtualKubelet.
func VirtualKubeletServiceAccountName(virtualNodeName string) string {
	return vkNamePrefix + virtualNodeName
}

// VirtualKubeletServiceAccount forges a ServiceAccount for a VirtualKubelet.
func VirtualKubeletServiceAccount(virtualNode *offloadingv1beta1.VirtualNode) *v1.ServiceAccount {
	return &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VirtualKubeletServiceAccountName(virtualNode.Name),
			Namespace: virtualNode.Namespace,
		},
	}
}
