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
	"k8s.io/utils/strings"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

// createVirtualKubeletDeployment creates the VirtualKubelet Deployment.
func (r *VirtualNodeReconciler) ensureVirtualKubeletDeploymentPresence(
	ctx context.Context, cl client.Client, virtualNode *virtualkubeletv1alpha1.VirtualNode) error {
	if err := UpdateCondition(ctx, cl, virtualNode,
		VkConditionMap{
			virtualkubeletv1alpha1.VirtualKubeletConditionType: VkCondition{
				Status: virtualkubeletv1alpha1.CreatingConditionStatusType,
			},
			virtualkubeletv1alpha1.NodeConditionType: VkCondition{
				Status: virtualkubeletv1alpha1.CreatingConditionStatusType,
			},
		},
	); err != nil {
		return err
	}

	namespace := virtualNode.Namespace
	name := virtualNode.Name
	remoteClusterIdentity := virtualNode.Spec.ClusterIdentity
	// create the base resources
	vkServiceAccount := vkforge.VirtualKubeletServiceAccount(namespace, name)
	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, vkServiceAccount, func() error {
		return nil
	})
	if err != nil {
		klog.Error(err)
		return err
	}
	klog.V(5).Infof("[%v] ServiceAccount %s/%s reconciled: %s",
		remoteClusterIdentity.ClusterName, vkServiceAccount.Namespace, vkServiceAccount.Name, op)

	vkClusterRoleBinding := vkforge.VirtualKubeletClusterRoleBinding(namespace, name, remoteClusterIdentity)
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
		VkConditionMap{
			virtualkubeletv1alpha1.VirtualKubeletConditionType: VkCondition{
				Status: virtualkubeletv1alpha1.RunningConditionStatusType,
			},
		}); err != nil {
		return err
	}
	return UpdateCondition(ctx, cl, virtualNode,
		VkConditionMap{
			virtualkubeletv1alpha1.NodeConditionType: VkCondition{
				Status: virtualkubeletv1alpha1.RunningConditionStatusType,
			},
		})
}

// ensureVirtualKubeletDeploymentAbsence deletes the VirtualKubelet Deployment.
func (r *VirtualNodeReconciler) ensureVirtualKubeletDeploymentAbsence(
	ctx context.Context, virtualNode *virtualkubeletv1alpha1.VirtualNode) (reEnque bool, err error) {
	virtualKubeletDeployment, err := r.getVirtualKubeletDeployment(ctx, virtualNode)
	if err != nil {
		klog.Error(err)
		return true, err
	}
	if virtualKubeletDeployment == nil {
		return false, nil
	}

	msg := fmt.Sprintf("[%v] Deleting virtual-kubelet in namespace %v", virtualNode.Spec.ClusterIdentity.ClusterID, virtualNode.Namespace)
	klog.Info(msg)
	r.EventsRecorder.Event(virtualNode, "Normal", "VkDeleted", msg)

	if err := r.Client.Delete(ctx, virtualKubeletDeployment); err != nil {
		klog.Error(err)
		return true, err
	}

	if ok, err := checkVirtualKubeletPodAbsence(ctx, r.Client, virtualNode, r.VirtualKubeletOptions); err != nil || !ok {
		return true, err
	}

	if err := r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{
		Name: strings.ShortenString(fmt.Sprintf("%s%s", vkMachinery.CRBPrefix, virtualNode.Name), 253),
	}}); err != nil {
		klog.Error(err)
		return true, err
	}

	if err := r.Client.Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name: virtualNode.Name, Namespace: virtualNode.Namespace,
	}}); err != nil {
		klog.Error(err)
		return true, err
	}

	return false, nil
}

// getVirtualKubeletDeployment returns the VirtualKubelet Deployment given a VirtualNode.
func (r *VirtualNodeReconciler) getVirtualKubeletDeployment(
	ctx context.Context, virtualNode *virtualkubeletv1alpha1.VirtualNode) (*appsv1.Deployment, error) {
	var deployList appsv1.DeploymentList
	labels := vkforge.VirtualKubeletLabels(virtualNode, r.VirtualKubeletOptions)
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

func checkVirtualKubeletPodAbsence(ctx context.Context, cl client.Client,
	vn *virtualkubeletv1alpha1.VirtualNode, vkopt *vkforge.VirtualKubeletOpts) (bool, error) {
	klog.Warningf("[%v] checking virtual-kubelet pod absence", vn.Spec.ClusterIdentity.ClusterID)
	list := &corev1.PodList{}
	labels := vkforge.VirtualKubeletLabels(vn, vkopt)
	err := cl.List(ctx, list, client.MatchingLabels(labels))
	if err != nil {
		klog.Error(err)
		return false, nil
	}
	return len(list.Items) == 0, err
}
