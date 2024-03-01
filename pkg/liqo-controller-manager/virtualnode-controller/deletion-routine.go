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

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
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
func (dr *DeletionRoutine) EnsureNodeAbsence(vn *virtualkubeletv1alpha1.VirtualNode) error {
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
	vn := &virtualkubeletv1alpha1.VirtualNode{}
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
			virtualkubeletv1alpha1.NodeConditionType: VnCondition{
				Status: virtualkubeletv1alpha1.DrainingConditionStatusType,
			}})

	var node *corev1.Node
	node, err = getters.GetNodeFromVirtualNode(ctx, dr.vnr.Client, vn)
	if client.IgnoreNotFound(err) != nil {
		err = fmt.Errorf("error getting node: %w", err)
		return err
	}

	if node != nil {
		if !*vn.Spec.CreateNode {
			// We need to ensure that the current pods will no recreate the node after deleting it.
			var found bool
			if found, err = vkutils.CheckVirtualKubeletFlagsConsistence(
				ctx, dr.vnr.Client, vn, dr.vnr.VirtualKubeletOptions, createNodeFalseFlag); err != nil || !found {
				if err == nil {
					err = fmt.Errorf("virtual kubelet pods are still running with arg %s", createNodeFalseFlag.String())
					return err
				}
				err = fmt.Errorf("error checking virtual kubelet pods: %w", err)
				return err
			}
		}
		if err = dr.deleteNode(ctx, node, vn); err != nil {
			err = fmt.Errorf("error deleting node: %w", err)
			return err
		}
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
		// Node is being deleted, but the VirtualNode resource is not.
		// The VirtualNode .Spec.CreateNode field is set to false.
		ForgeCondition(vn, VnConditionMap{
			virtualkubeletv1alpha1.NodeConditionType: VnCondition{
				Status: virtualkubeletv1alpha1.NoneConditionStatusType,
			}})
	}

	klog.Infof("Deletion routine completed for virtual node %s", vn.Name)
	return err
}

// deleteNode deletes the Node created by VirtualNode.
func (dr *DeletionRoutine) deleteNode(ctx context.Context, node *corev1.Node, vn *virtualkubeletv1alpha1.VirtualNode) error {
	if err := cordonNode(ctx, dr.vnr.Client, node); err != nil {
		return fmt.Errorf("error cordoning node: %w", err)
	}

	klog.Infof("Node %s cordoned", node.Name)

	if err := client.IgnoreNotFound(drainNode(ctx, dr.vnr.Client, vn)); err != nil {
		return fmt.Errorf("error draining node: %w", err)
	}

	klog.Infof("Node %s drained", node.Name)

	if !vn.DeletionTimestamp.IsZero() {
		ForgeCondition(vn,
			VnConditionMap{
				virtualkubeletv1alpha1.VirtualKubeletConditionType: VnCondition{
					Status: virtualkubeletv1alpha1.DeletingConditionStatusType,
				},
			},
		)
		if err := dr.vnr.ensureVirtualKubeletDeploymentAbsence(ctx, vn); err != nil {
			return fmt.Errorf("error deleting virtual kubelet deployment: %w", err)
		}
	}
	klog.Infof("VirtualKubelet deployment %s deleted", vn.Name)

	ForgeCondition(vn,
		VnConditionMap{
			virtualkubeletv1alpha1.NodeConditionType: VnCondition{
				Status: virtualkubeletv1alpha1.DeletingConditionStatusType,
			},
		})
	if err := client.IgnoreNotFound(dr.vnr.Client.Delete(ctx, node, &client.DeleteOptions{})); err != nil {
		return fmt.Errorf("error deleting node: %w", err)
	}

	klog.Infof("Node %s deleted", node.Name)
	return nil
}
