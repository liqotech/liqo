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

package virtualnodectrl

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	k8strings "k8s.io/utils/strings"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
	vkutils "github.com/liqotech/liqo/pkg/vkMachinery/utils"
)

// createVirtualKubeletDeployment creates the VirtualKubelet Deployment.
func (r *VirtualNodeReconciler) ensureVirtualKubeletDeploymentPresence(
	ctx context.Context, virtualNode *virtualkubeletv1alpha1.VirtualNode) (err error) {
	var nodeStatusInitial virtualkubeletv1alpha1.VirtualNodeConditionStatusType
	if *virtualNode.Spec.CreateNode {
		nodeStatusInitial = virtualkubeletv1alpha1.CreatingConditionStatusType
	} else {
		nodeStatusInitial = virtualkubeletv1alpha1.NoneConditionStatusType
	}
	defer func() {
		if interr := r.Client.Status().Update(ctx, virtualNode); interr != nil {
			if err != nil {
				klog.Error(err)
			}
			err = fmt.Errorf("error updating virtual node status: %w", interr)
		}
	}()
	ForgeCondition(virtualNode,
		VnConditionMap{
			virtualkubeletv1alpha1.VirtualKubeletConditionType: VnCondition{
				Status: virtualkubeletv1alpha1.CreatingConditionStatusType,
			},
			virtualkubeletv1alpha1.NodeConditionType: VnCondition{Status: nodeStatusInitial},
		},
	)

	namespace := virtualNode.Namespace
	name := virtualNode.Name
	remoteClusterIdentity := virtualNode.Spec.ClusterIdentity
	// create the base resources
	vkServiceAccount := vkforge.VirtualKubeletServiceAccount(namespace, name)
	var op controllerutil.OperationResult
	op, err = controllerutil.CreateOrUpdate(ctx, r.Client, vkServiceAccount, func() error {
		return nil
	})
	if err != nil {
		return err
	}
	klog.V(5).Infof("[%v] ServiceAccount %s/%s reconciled: %s",
		remoteClusterIdentity.ClusterName, vkServiceAccount.Namespace, vkServiceAccount.Name, op)

	vkClusterRoleBinding := vkforge.VirtualKubeletClusterRoleBinding(namespace, name, remoteClusterIdentity)
	op, err = controllerutil.CreateOrUpdate(ctx, r.Client, vkClusterRoleBinding, func() error {
		return nil
	})
	if err != nil {
		return err
	}

	klog.V(5).Infof("[%v] ClusterRoleBinding %s reconciled: %s",
		remoteClusterIdentity.ClusterName, vkClusterRoleBinding.Name, op)

	// forge the virtual Kubelet Deployment
	vkDeployment := &appsv1.Deployment{}
	vkDeployment.ObjectMeta = *virtualNode.Spec.Template.ObjectMeta.DeepCopy()

	op, err = controllerutil.CreateOrUpdate(ctx, r.Client, vkDeployment, func() error {
		vkDeployment.Spec = *virtualNode.Spec.Template.Spec.DeepCopy()
		return nil
	})
	if err != nil {
		return err
	}
	klog.V(5).Infof("[%v] Deployment %s/%s reconciled: %s",
		remoteClusterIdentity.ClusterName, vkDeployment.Namespace, vkDeployment.Name, op)

	if op == controllerutil.OperationResultCreated {
		msg := fmt.Sprintf("[%v] Launching virtual-kubelet %s in namespace %v",
			remoteClusterIdentity.ClusterName, vkDeployment.Name, namespace)
		klog.Info(msg)
		r.EventsRecorder.Event(virtualNode, "Normal", "VkCreated", msg)
	}

	ForgeCondition(virtualNode,
		VnConditionMap{
			virtualkubeletv1alpha1.VirtualKubeletConditionType: VnCondition{
				Status: virtualkubeletv1alpha1.RunningConditionStatusType,
			}})
	if *virtualNode.Spec.CreateNode {
		ForgeCondition(virtualNode,
			VnConditionMap{
				virtualkubeletv1alpha1.NodeConditionType: VnCondition{
					Status: virtualkubeletv1alpha1.RunningConditionStatusType,
				},
			})
	}
	return err
}

// ensureVirtualKubeletDeploymentAbsence deletes the VirtualKubelet Deployment.
// It checks if the VirtualKubelet Pods have been deleted.
func (r *VirtualNodeReconciler) ensureVirtualKubeletDeploymentAbsence(
	ctx context.Context, virtualNode *virtualkubeletv1alpha1.VirtualNode) error {
	virtualKubeletDeployment, err := vkutils.GetVirtualKubeletDeployment(ctx, r.Client, virtualNode, r.VirtualKubeletOptions)
	if err != nil {
		return err
	}
	if virtualKubeletDeployment != nil {
		msg := fmt.Sprintf("[%v] Deleting virtual-kubelet in namespace %v", virtualNode.Spec.ClusterIdentity.ClusterID, virtualNode.Namespace)
		klog.Info(msg)
		r.EventsRecorder.Event(virtualNode, "Normal", "VkDeleted", msg)

		if err := r.Client.Delete(ctx, virtualKubeletDeployment); err != nil {
			return err
		}
	}

	if err := vkutils.CheckVirtualKubeletPodAbsence(ctx, r.ClientVK, virtualNode, r.VirtualKubeletOptions); err != nil {
		return err
	}

	err = r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{
		Name: k8strings.ShortenString(fmt.Sprintf("%s%s", vkMachinery.CRBPrefix, virtualNode.Name), 253),
	}})
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	err = r.Client.Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name: virtualNode.Name, Namespace: virtualNode.Namespace,
	}})
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}
