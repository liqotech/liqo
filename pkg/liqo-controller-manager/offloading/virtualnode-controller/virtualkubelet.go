// Copyright 2019-2025 The Liqo Authors
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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	k8strings "k8s.io/utils/strings"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/resource"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
	vkutils "github.com/liqotech/liqo/pkg/vkMachinery/utils"
)

const offloadingPatchHashAnnotation = "liqo.io/offloading-patch-hash"

// createVirtualKubeletDeployment creates the VirtualKubelet Deployment.
func (r *VirtualNodeReconciler) ensureVirtualKubeletDeploymentPresence(
	ctx context.Context, virtualNode *offloadingv1beta1.VirtualNode) (err error) {
	var nodeStatusInitial offloadingv1beta1.VirtualNodeConditionStatusType
	if *virtualNode.Spec.CreateNode {
		nodeStatusInitial = offloadingv1beta1.CreatingConditionStatusType
	} else {
		nodeStatusInitial = offloadingv1beta1.NoneConditionStatusType
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
			offloadingv1beta1.VirtualKubeletConditionType: VnCondition{
				Status: offloadingv1beta1.CreatingConditionStatusType,
			},
			offloadingv1beta1.NodeConditionType: VnCondition{Status: nodeStatusInitial},
		},
	)

	namespace := virtualNode.Namespace
	name := virtualNode.Name
	remoteClusterID := virtualNode.Spec.ClusterID
	// create the base resources
	vkServiceAccount := vkforge.VirtualKubeletServiceAccount(namespace, name)
	var op controllerutil.OperationResult
	op, err = resource.CreateOrUpdate(ctx, r.Client, vkServiceAccount, func() error {
		return nil
	})
	if err != nil {
		return err
	}
	klog.V(5).Infof("[%v] ServiceAccount %s/%s reconciled: %s",
		remoteClusterID, vkServiceAccount.Namespace, vkServiceAccount.Name, op)

	vkClusterRoleBinding := vkforge.VirtualKubeletClusterRoleBinding(namespace, name, remoteClusterID)
	op, err = resource.CreateOrUpdate(ctx, r.Client, vkClusterRoleBinding, func() error {
		return nil
	})
	if err != nil {
		return err
	}

	klog.V(5).Infof("[%v] ClusterRoleBinding %s reconciled: %s",
		remoteClusterID, vkClusterRoleBinding.Name, op)

	// forge the virtual Kubelet Deployment
	vkDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualNode.Spec.Template.GetName(),
			Namespace: virtualNode.Spec.Template.GetNamespace(),
		},
	}
	op, err = resource.CreateOrUpdate(ctx, r.Client, &vkDeployment, func() error {
		vkDeployment.Annotations = labels.Merge(vkDeployment.Annotations, virtualNode.Spec.Template.ObjectMeta.GetAnnotations())
		vkDeployment.Labels = labels.Merge(vkDeployment.Labels, virtualNode.Spec.Template.ObjectMeta.GetLabels())

		vkDeployment.Spec = *virtualNode.Spec.Template.Spec.DeepCopy()

		// Add the hash of the offloading patch as annotation
		opHash, err := offloadingPatchHash(virtualNode.Spec.OffloadingPatch)
		if err != nil {
			return err
		}
		if vkDeployment.Spec.Template.Annotations == nil {
			vkDeployment.Spec.Template.Annotations = make(map[string]string)
		}
		vkDeployment.Spec.Template.Annotations[offloadingPatchHashAnnotation] = opHash

		return nil
	})
	if err != nil {
		return err
	}
	klog.V(5).Infof("[%v] Deployment %s/%s reconciled: %s",
		remoteClusterID, vkDeployment.Namespace, vkDeployment.Name, op)

	if op == controllerutil.OperationResultCreated {
		msg := fmt.Sprintf("[%v] Launching virtual-kubelet %s in namespace %v",
			remoteClusterID, vkDeployment.Name, namespace)
		klog.Info(msg)
		r.EventsRecorder.Event(virtualNode, "Normal", "VkCreated", msg)
	}

	ForgeCondition(virtualNode,
		VnConditionMap{
			offloadingv1beta1.VirtualKubeletConditionType: VnCondition{
				Status: offloadingv1beta1.RunningConditionStatusType,
			},
		})

	if *virtualNode.Spec.CreateNode {
		ForgeCondition(virtualNode,
			VnConditionMap{
				offloadingv1beta1.NodeConditionType: VnCondition{
					Status: offloadingv1beta1.RunningConditionStatusType,
				},
			})
	}
	return err
}

// ensureVirtualKubeletDeploymentAbsence deletes the VirtualKubelet Deployment.
// It checks if the VirtualKubelet Pods have been deleted.
func (r *VirtualNodeReconciler) ensureVirtualKubeletDeploymentAbsence(
	ctx context.Context, virtualNode *offloadingv1beta1.VirtualNode) error {
	virtualKubeletDeployment, err := vkutils.GetVirtualKubeletDeployment(ctx, r.Client, virtualNode)
	if err != nil {
		return err
	}
	if virtualKubeletDeployment != nil {
		msg := fmt.Sprintf("[%v] Deleting virtual-kubelet in namespace %v", virtualNode.Spec.ClusterID, virtualNode.Namespace)
		klog.Info(msg)
		r.EventsRecorder.Event(virtualNode, "Normal", "VkDeleted", msg)

		if err := r.Client.Delete(ctx, virtualKubeletDeployment); err != nil {
			return err
		}
	}

	if err := vkutils.CheckVirtualKubeletPodAbsence(ctx, r.Client, virtualNode); err != nil {
		return err
	}

	crbName := k8strings.ShortenString(fmt.Sprintf("%s%s", vkMachinery.CRBPrefix, virtualNode.Name), 253)
	err = r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{
		Name: crbName,
	}})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	klog.Info(fmt.Sprintf("[%v] Deleted virtual-kubelet CRB %s", virtualNode.Spec.ClusterID, crbName))

	err = r.Client.Delete(ctx, &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name: virtualNode.Name, Namespace: virtualNode.Namespace,
	}})
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

func offloadingPatchHash(offloadingPatch *offloadingv1beta1.OffloadingPatch) (string, error) {
	if offloadingPatch == nil {
		return "", nil
	}

	opString, err := json.Marshal(offloadingPatch)
	if err != nil {
		klog.Error(err)
		return "", err
	}

	opHash := sha256.Sum256(opString)
	opHashHex := hex.EncodeToString(opHash[:])

	return opHashHex, nil
}
