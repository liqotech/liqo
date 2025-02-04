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

// Package virtualnodectrl contains VirtualNode Controller logic and some functions for managing NamespaceMap lifecycle.
// There are also some tests for VirtualNode Controller
package virtualnodectrl

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/getters"
	vkMachineryForge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
	vkutils "github.com/liqotech/liqo/pkg/vkMachinery/utils"
)

var (
	deletionRoutineRunning = false
	createNodeFalseFlag    = vkutils.Flag{
		Name:  string(vkMachineryForge.CreateNode),
		Value: strconv.FormatBool(false),
	}
)

// DeletionRoutine is responsible for deleting a virtual node.
type DeletionRoutine struct {
	vnr *VirtualNodeReconciler
	wq  workqueue.RateLimitingInterface
}

// RunDeletionRoutine starts the deletion routine.
func RunDeletionRoutine(ctx context.Context, r *VirtualNodeReconciler) (*DeletionRoutine, error) {
	if deletionRoutineRunning {
		return nil, fmt.Errorf("deletion routine already running")
	}
	deletionRoutineRunning = true
	dr := &DeletionRoutine{
		vnr: r,
		wq:  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}
	go dr.run(ctx)
	return dr, nil
}

// EnsureNodeAbsence adds a virtual node to the deletion queue.
func (dr *DeletionRoutine) EnsureNodeAbsence(vn *offloadingv1beta1.VirtualNode) error {
	key, err := cache.MetaNamespaceKeyFunc(vn)
	if err != nil {
		return fmt.Errorf("error getting key: %w", err)
	}
	dr.wq.AddRateLimited(key)
	return nil
}

func (dr *DeletionRoutine) run(ctx context.Context) {
	defer klog.Error("Deletion routine stopped")
	for dr.processNextItem(ctx) {
	}
}

func (dr *DeletionRoutine) processNextItem(ctx context.Context) bool {
	var err error
	key, quit := dr.wq.Get()
	if quit {
		return false
	}
	defer dr.wq.Done(key)

	ref, ok := key.(string)
	if !ok {
		klog.Errorf("expected string in workqueue but got %#v", key)
		return true
	}

	if err = dr.handle(ctx, ref); err == nil {
		dr.wq.Forget(key)
		return true
	}

	klog.Errorf("error processing %q (will retry): %v", key, err)
	dr.wq.AddRateLimited(key)
	return true
}

func (dr *DeletionRoutine) handle(ctx context.Context, key string) (err error) {
	var namespace, name string
	namespace, name, err = cache.SplitMetaNamespaceKey(key)
	if err != nil {
		err = fmt.Errorf("error splitting key: %w", err)
		return err
	}
	ref := types.NamespacedName{Namespace: namespace, Name: name}
	vn := &offloadingv1beta1.VirtualNode{}
	if err = dr.vnr.Client.Get(ctx, ref, vn); err != nil {
		if k8serrors.IsNotFound(err) {
			err = nil
			return err
		}
		err = fmt.Errorf("error getting virtual node: %w", err)
		return err
	}

	defer func() {
		if interr := dr.vnr.Client.Status().Update(ctx, vn); interr != nil {
			if err != nil {
				klog.Error(err)
			}
			err = fmt.Errorf("error updating virtual node status: %w", interr)
		}
	}()

	klog.Infof("Deletion routine started for virtual node %s", vn.Name)
	ForgeCondition(vn,
		VnConditionMap{
			offloadingv1beta1.NodeConditionType: VnCondition{
				Status: offloadingv1beta1.DrainingConditionStatusType,
			}})

	if err = dr.deleteVirtualKubelet(ctx, vn); err != nil {
		err = fmt.Errorf("error deleting node and Virtual Kubelet: %w", err)
		return err
	}

	if !vn.DeletionTimestamp.IsZero() {
		// VirtualNode resource is being deleted.
		if err = dr.vnr.ensureNamespaceMapAbsence(ctx, vn); err != nil {
			err = fmt.Errorf("error deleting namespace map: %w", err)
			return err
		}
		err = dr.vnr.removeVirtualNodeFinalizer(ctx, vn)
		if err != nil {
			err = fmt.Errorf("error removing finalizer: %w", err)
			return err
		}
	} else {
		// Node is deleting/deleted, but the VirtualNode resource is not
		// (the virtualNode .Spec.CreateNode field is set to false).
		ForgeCondition(vn, VnConditionMap{
			offloadingv1beta1.NodeConditionType: VnCondition{
				Status: offloadingv1beta1.NoneConditionStatusType,
			}})
	}

	klog.Infof("Deletion routine completed for virtual node %s", vn.Name)
	return nil
}

// deleteVirtualKubelet deletes the Node and the VirtualKubelet deployment related to the given VirtualNode.
func (dr *DeletionRoutine) deleteVirtualKubelet(ctx context.Context, vn *offloadingv1beta1.VirtualNode) error {
	// Check if the Node resource exists to make sure that we are not in a case in which it should not exist.
	node, err := getters.GetNodeFromVirtualNode(ctx, dr.vnr.Client, vn)
	if client.IgnoreNotFound(err) != nil {
		err = fmt.Errorf("error getting node: %w", err)
		return err
	}

	if node != nil {
		if err := cordonNode(ctx, dr.vnr.Client, node); err != nil {
			return fmt.Errorf("error cordoning node: %w", err)
		}

		klog.Infof("Node %s cordoned", node.Name)

		if err := client.IgnoreNotFound(drainNode(ctx, dr.vnr.Client, vn)); err != nil {
			return fmt.Errorf("error draining node: %w", err)
		}

		klog.Infof("Node %s drained", node.Name)
	}

	if !vn.DeletionTimestamp.IsZero() {
		ForgeCondition(vn,
			VnConditionMap{
				offloadingv1beta1.VirtualKubeletConditionType: VnCondition{
					Status: offloadingv1beta1.DeletingConditionStatusType,
				},
			},
		)
		if err := dr.vnr.ensureVirtualKubeletDeploymentAbsence(ctx, vn); err != nil {
			return fmt.Errorf("error deleting virtual kubelet deployment: %w", err)
		}
	}

	// Even node is nil we make sure that no Node resource has been created before the deletion of the VK deployment.
	klog.Infof("VirtualKubelet deployment %s deleted", vn.Name)

	var nodeToDelete *corev1.Node

	if node != nil {
		nodeToDelete = node
	} else {
		nodeToDelete, err = getters.GetNodeFromVirtualNode(ctx, dr.vnr.Client, vn)
		if client.IgnoreNotFound(err) != nil {
			err = fmt.Errorf("error getting node before deletion: %w", err)
			return err
		}
	}

	ForgeCondition(vn,
		VnConditionMap{
			offloadingv1beta1.NodeConditionType: VnCondition{
				Status: offloadingv1beta1.DeletingConditionStatusType,
			},
		})

	if nodeToDelete != nil {
		if err := client.IgnoreNotFound(dr.vnr.Client.Delete(ctx, nodeToDelete, &client.DeleteOptions{})); err != nil {
			return fmt.Errorf("error deleting node: %w", err)
		}

		klog.Infof("Node %s deleted", node.Name)
	} else {
		klog.Infof("Node of VirtualNode %s already deleted", vn.Name)
	}

	return nil
}
