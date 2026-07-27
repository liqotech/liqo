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

package virtualnodectrl

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	k8strings "k8s.io/utils/strings"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	mapsutil "github.com/liqotech/liqo/pkg/utils/maps"
	"github.com/liqotech/liqo/pkg/utils/resource"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
	vkutils "github.com/liqotech/liqo/pkg/vkMachinery/utils"
)

const offloadingPatchHashAnnotation = "liqo.io/offloading-patch-hash"

// getVkOptionsTemplate returns the VkOptionsTemplate referenced by the given VirtualNode,
// falling back to the default one when no explicit reference is set.
func (r *VirtualNodeReconciler) getVkOptionsTemplate(ctx context.Context,
	virtualNode *offloadingv1beta1.VirtualNode) (*offloadingv1beta1.VkOptionsTemplate, error) {
	ref := virtualNode.Spec.VkOptionsTemplateRef
	if ref == nil {
		ref = r.VkOptsDefaultTemplate
	}
	if ref == nil {
		r.EventsRecorder.Event(virtualNode, "Warning", "NoVkOptionsTemplate",
			"No VkOptionsTemplate reference set on this virtual node, and no default template configured on the controller")
		return nil, fmt.Errorf("no VkOptionsTemplate reference set on the virtual-node %q, and no default configured",
			client.ObjectKeyFromObject(virtualNode))
	}

	refKey := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	vkOpts := &offloadingv1beta1.VkOptionsTemplate{}
	if err := r.Get(ctx, refKey, vkOpts); err != nil {
		return nil, fmt.Errorf("getting the VkOptionsTemplate %q: %w", refKey, err)
	}
	return vkOpts, nil
}

// ensureVirtualKubeletDeploymentPresence creates or updates the VirtualKubelet Deployment,
// deterministically forging it from the given VirtualNode and VkOptionsTemplate.
func (r *VirtualNodeReconciler) ensureVirtualKubeletDeploymentPresence(
	ctx context.Context, virtualNode *offloadingv1beta1.VirtualNode,
	vkOpts *offloadingv1beta1.VkOptionsTemplate) (err error) {
	var nodeStatusInitial offloadingv1beta1.VirtualNodeConditionStatusType
	if vkforge.EffectiveCreateNode(virtualNode, vkOpts) {
		nodeStatusInitial = offloadingv1beta1.CreatingConditionStatusType
	} else {
		nodeStatusInitial = offloadingv1beta1.NoneConditionStatusType
	}
	defer func() {
		if interr := r.Client.Status().Update(ctx, virtualNode); interr != nil {
			err = errors.Join(err, fmt.Errorf("updating virtual node status: %w", interr))
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

	// Publish the effective offloading patch (the spec's one with the NotReflected lists
	// merged with the VkOptionsTemplate ones) in the status: the virtual-kubelet reads it
	// at startup, preferring it over the spec one.
	virtualNode.Status.EffectiveOffloadingPatch = vkforge.EffectiveOffloadingPatch(virtualNode, vkOpts)
	// Publish the effective createNode and disableNetworkCheck values (defaulting to the
	// VkOptionsTemplate ones when not set in the spec) in the status, for observability.
	virtualNode.Status.EffectiveCreateNode = ptr.To(vkforge.EffectiveCreateNode(virtualNode, vkOpts))
	virtualNode.Status.EffectiveDisableNetworkCheck = ptr.To(vkforge.EffectiveDisableNetworkCheck(virtualNode, vkOpts))

	namespace := virtualNode.Namespace
	name := virtualNode.Name
	remoteClusterID := virtualNode.Spec.ClusterID

	vkServiceAccount := vkforge.VirtualKubeletServiceAccount(virtualNode)
	var op controllerutil.OperationResult
	op, err = resource.CreateOrUpdate(ctx, r.Client, vkServiceAccount, func() error {
		// Set the VirtualNode as controller owner, so that the garbage collector deletes
		// the ServiceAccount upon VirtualNode deletion.
		if err := controllerutil.SetControllerReference(virtualNode, vkServiceAccount, r.Scheme); err != nil {
			return fmt.Errorf("setting the owner reference on the virtual-kubelet ServiceAccount: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("enforcing virtual-kubelet ServiceAccount: %w", err)
	}
	klog.V(5).Infof("[%v] ServiceAccount %s/%s reconciled: %s",
		remoteClusterID, vkServiceAccount.Namespace, vkServiceAccount.Name, op)

	vkClusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: vkforge.VirtualKubeletClusterRoleBindingName(name),
		},
	}
	op, err = resource.CreateOrUpdate(ctx, r.Client, vkClusterRoleBinding,
		vkforge.VirtualKubeletClusterRoleBindingMutateFn(vkClusterRoleBinding, virtualNode))
	if err != nil {
		return fmt.Errorf("enforcing virtual-kubelet ClusterRoleBinding: %w", err)
	}

	klog.V(5).Infof("[%v] ClusterRoleBinding %s reconciled: %s",
		remoteClusterID, vkClusterRoleBinding.Name, op)

	// Bind the virtual kubelet service account to system:auth-delegator, to allow it to perform
	// TokenAccessReview and SubjectAccessReview and secure the virtual kubelet API with webhook auth.
	vkAuthDelegatorCRB := vkforge.VirtualKubeletAuthDelegatorClusterRoleBinding(namespace, name, remoteClusterID)
	op, err = resource.CreateOrUpdate(ctx, r.Client, vkAuthDelegatorCRB, func() error {
		return nil
	})
	if err != nil {
		return err
	}

	klog.V(5).Infof("[%v] Auth delegator ClusterRoleBinding %s reconciled: %s",
		remoteClusterID, vkAuthDelegatorCRB.Name, op)

	// forge the VirtualKubelet Deployment from the VirtualNode and the VkOptionsTemplate.
	forgedDeployment := vkforge.VirtualKubeletDeployment(r.HomeClusterID, r.LiqoNamespace, r.LocalPodCIDRs, virtualNode, vkOpts)
	vkDeployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forgedDeployment.Name,
			Namespace: forgedDeployment.Namespace,
		},
	}
	op, err = resource.CreateOrUpdate(ctx, r.Client, &vkDeployment, func() error {
		mapsutil.SmartMergeLabels(&vkDeployment, forgedDeployment.Labels)
		mapsutil.SmartMergeAnnotations(&vkDeployment, forgedDeployment.Annotations)

		// Preserve the pod template annotations and the paused flag, which are not managed
		// by the forge (e.g., the kubectl.kubernetes.io/restartedAt annotation added by
		// `kubectl rollout restart`, and `kubectl rollout pause`), before enforcing the
		// forged spec: reverting them would fight the operators' rollout actions.
		templateAnnotations := vkDeployment.Spec.Template.Annotations
		paused := vkDeployment.Spec.Paused
		vkDeployment.Spec = *forgedDeployment.Spec.DeepCopy() // this override the whole spec
		vkDeployment.Spec.Paused = paused
		vkDeployment.Spec.Template.Annotations = templateAnnotations
		mapsutil.SmartMergeAnnotations(&vkDeployment.Spec.Template.ObjectMeta, forgedDeployment.Spec.Template.Annotations)

		// Set the VirtualNode as controller owner, to ensure garbage collection
		// even in case the finalizer is not correctly removed.
		if err := controllerutil.SetControllerReference(virtualNode, &vkDeployment, r.Scheme); err != nil {
			return fmt.Errorf("setting the owner reference on the virtual-kubelet Deployment: %w", err)
		}

		// Add the hash of the effective offloading patch as annotation, to restart the
		// virtual-kubelet whenever the fields read at startup change (including the
		// NotReflected lists contributed by the VkOptionsTemplate).
		opHash, err := offloadingPatchHash(virtualNode.Status.EffectiveOffloadingPatch)
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
		return fmt.Errorf("enforcing virtual-kubelet Deployment: %w", err)
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

	if vkforge.EffectiveCreateNode(virtualNode, vkOpts) {
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

	crbName := vkforge.VirtualKubeletClusterRoleBindingName(virtualNode.Name)
	err = r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{
		Name: crbName,
	}})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	klog.Info(fmt.Sprintf("[%v] Deleted virtual-kubelet CRB %s", virtualNode.Spec.ClusterID, crbName))

	authDelegatorCRBName := k8strings.ShortenString(fmt.Sprintf("%s%s", vkMachinery.AuthDelegatorCRBPrefix, virtualNode.Name), 253)
	err = r.Client.Delete(ctx, &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{
		Name: authDelegatorCRBName,
	}})
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	klog.Info(fmt.Sprintf("[%v] Deleted virtual-kubelet auth delegator CRB %s", virtualNode.Spec.ClusterID, authDelegatorCRBName))

	// The ServiceAccount is garbage collected by the owner reference, no need to delete it.

	return nil
}

func offloadingPatchHash(offloadingPatch *offloadingv1beta1.OffloadingPatch) (string, error) {
	if offloadingPatch == nil {
		return "", nil
	}

	opString, err := json.Marshal(offloadingPatch)
	if err != nil {
		return "", fmt.Errorf("marshaling offloading patch: %w", err)
	}

	opHash := sha256.Sum256(opString)
	opHashHex := hex.EncodeToString(opHash[:])

	return opHashHex, nil
}
