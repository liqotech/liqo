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

package liqonodeprovider

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

var (
	// csinodeEnsureTimeout is the maximum time spent trying to set up the CSINode object.
	// It covers the window in which the CSINode owned by a previous incarnation of the node (same name, different UID)
	// has not been garbage collected yet.
	csinodeEnsureTimeout = 30 * time.Second
	// csinodeEnsurePollInterval is the interval between two attempts to set up the CSINode object.
	csinodeEnsurePollInterval = 2 * time.Second
)

// EnsureCSINode makes sure that the CSINode object associated with the virtual node exists and is controlled by the
// current node object. The virtual kubelet installs no CSI driver, hence the drivers list is empty.
// An object already present without a controller owner reference (e.g. manually created) is tolerated and left
// untouched, hence it is not garbage collected together with the node.
func (p *LiqoNodeProvider) EnsureCSINode(ctx context.Context) error {
	var lastErr error
	err := wait.PollUntilContextTimeout(ctx, csinodeEnsurePollInterval, csinodeEnsureTimeout, true,
		func(ctx context.Context) (bool, error) {
			if lastErr = p.ensureCSINode(ctx); lastErr != nil {
				klog.Warningf("Failed to set up the CSINode for node %q, retrying: %v", p.nodeName, lastErr)
				return false, nil
			}
			return true, nil
		})
	if err != nil && lastErr != nil {
		return lastErr
	}
	return err
}

// ensureCSINode performs a single attempt to set up the CSINode object, depending on its current state.
func (p *LiqoNodeProvider) ensureCSINode(ctx context.Context) error {
	// The node object coming from the provider may not have the UID yet, hence we read it from the API server.
	node, err := p.localClient.CoreV1().Nodes().Get(ctx, p.nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("retrieving node %q to forge the associated CSINode: %w", p.nodeName, err)
	}

	_, err = p.localClient.StorageV1().CSINodes().Create(ctx, forgeCSINode(node), metav1.CreateOptions{})
	switch {
	case err == nil:
		klog.Infof("CSINode object %q correctly created", node.GetName())
		return nil
	case !apierrors.IsAlreadyExists(err):
		return fmt.Errorf("creating CSINode object for node %q: %w", node.GetName(), err)
	}

	csiNode, err := p.localClient.StorageV1().CSINodes().Get(ctx, node.GetName(), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("retrieving the existing CSINode object %q: %w", node.GetName(), err)
	}

	ownerRef := metav1.GetControllerOf(csiNode)
	switch {
	case ownerRef == nil:
		// The object was created without an owner reference (e.g. manually): it is tolerated and left untouched,
		// hence it is not garbage collected together with the node.
		klog.Infof("CSINode object %q exists without an owner reference, tolerating it", csiNode.GetName())
	case ownerRef.UID == node.GetUID():
		klog.V(4).Infof("CSINode object %q correctly already exists", csiNode.GetName())
	default:
		return fmt.Errorf("the CSINode object %q already exists but is not controlled by node %q (uid %q): "+
			"stale owner reference, waiting for garbage collection", csiNode.GetName(), node.GetName(), node.GetUID())
	}

	return nil
}

func forgeCSINode(node *corev1.Node) *storagev1.CSINode {
	return &storagev1.CSINode{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.GetName(),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Node",
				Name:       node.GetName(),
				UID:        node.GetUID(),
				Controller: ptr.To(true),
			}},
		},
		Spec: storagev1.CSINodeSpec{Drivers: []storagev1.CSINodeDriver{}},
	}
}
