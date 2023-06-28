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
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

// createVirtualKubeletDeployment creates the VirtualKubelet Deployment.
func (r *VirtualNodeReconciler) ensureVirtualKubeletDeploymentPresence(
	ctx context.Context, cl client.Client, virtualNode *virtualkubeletv1alpha1.VirtualNode) error {
	if err := UpdateCondition(ctx, cl, virtualNode,
		virtualkubeletv1alpha1.VirtualKubeletConditionType, virtualkubeletv1alpha1.CreatingConditionStatusType); err != nil {
		return err
	}
	if err := UpdateCondition(ctx, cl, virtualNode,
		virtualkubeletv1alpha1.NodeConditionType, virtualkubeletv1alpha1.CreatingConditionStatusType); err != nil {
		return err
	}

	namespace := virtualNode.Namespace
	remoteClusterIdentity := virtualNode.Spec.ClusterIdentity
	// create the base resources
	vkServiceAccount := forge.VirtualKubeletServiceAccount(namespace)
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, vkServiceAccount, func() error {
		return nil
	})
	if err != nil {
		klog.Error(err)
		return err
	}
	klog.V(5).Infof("[%v] ServiceAccount %s/%s reconciled: %s",
		remoteClusterIdentity.ClusterName, vkServiceAccount.Namespace, vkServiceAccount.Name, op)

	vkClusterRoleBinding := forge.VirtualKubeletClusterRoleBinding(namespace, remoteClusterIdentity)
	op, err = controllerutil.CreateOrUpdate(ctx, r.Client, vkClusterRoleBinding, func() error {
		return nil
	})
	if err != nil {
		klog.Error(err)
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
		klog.Error(err)
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

	if err := UpdateCondition(ctx, cl, virtualNode,
		virtualkubeletv1alpha1.VirtualKubeletConditionType, virtualkubeletv1alpha1.RunningConditionStatusType); err != nil {
		return err
	}
	return UpdateCondition(ctx, cl, virtualNode,
		virtualkubeletv1alpha1.NodeConditionType, virtualkubeletv1alpha1.RunningConditionStatusType)
}

// ensureVirtualKubeletDeploymentAbsence deletes the VirtualKubelet Deployment.
func (r *VirtualNodeReconciler) ensureVirtualKubeletDeploymentAbsence(
	ctx context.Context, virtualNode *virtualkubeletv1alpha1.VirtualNode) error {
	virtualKubeletDeployment, err := r.getVirtualKubeletDeployment(ctx, virtualNode)
	if err != nil {
		klog.Error(err)
		return err
	}
	if virtualKubeletDeployment == nil {
		return nil
	}

	if err := r.Client.Delete(ctx, virtualKubeletDeployment); err != nil {
		klog.Error(err)
		return err
	}

	msg := fmt.Sprintf("[%v] Deleting virtual-kubelet in namespace %v", virtualNode.Spec.ClusterIdentity.ClusterID, virtualNode.Namespace)
	klog.Info(msg)
	r.EventsRecorder.Event(virtualNode, "Normal", "VkDeleted", msg)

	crlabels := forge.ClusterRoleLabels(virtualNode.Spec.ClusterIdentity.ClusterID)

	virtualnodes := &virtualkubeletv1alpha1.VirtualNodeList{}
	if err := r.Client.List(ctx, virtualnodes, client.MatchingLabels{discovery.ClusterIDLabel: virtualNode.Spec.ClusterIdentity.ClusterID}); err != nil {
		klog.Error(err)
		return err
	}

	if len(virtualnodes.Items) > 1 {
		return nil
	}

	if err := r.Client.DeleteAllOf(ctx, &rbacv1.ClusterRoleBinding{}, client.MatchingLabels(crlabels)); err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

// getVirtualKubeletDeployment returns the VirtualKubelet Deployment given a ResourceOffer.
func (r *VirtualNodeReconciler) getVirtualKubeletDeployment(
	ctx context.Context, virtualNode *virtualkubeletv1alpha1.VirtualNode) (*appsv1.Deployment, error) {
	var deployList appsv1.DeploymentList
	labels := forge.VirtualKubeletLabels(virtualNode, r.VirtualKubeletOptions)
	if err := r.Client.List(ctx, &deployList, client.MatchingLabels(labels)); err != nil {
		klog.Error(err)
		return nil, err
	}

	if len(deployList.Items) == 0 {
		klog.V(4).Infof("[%v] no VirtualKubelet deployment found", virtualNode.Spec.ClusterIdentity.ClusterID)
		return nil, nil
	} else if len(deployList.Items) > 1 {
		err := fmt.Errorf("[%v] more than one VirtualKubelet deployment found", virtualNode.Spec.ClusterIdentity.ClusterID)
		klog.Error(err)
		return nil, err
	}

	return &deployList.Items[0], nil
}
