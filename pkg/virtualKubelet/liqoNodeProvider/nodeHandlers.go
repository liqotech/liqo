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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// HandleNodeStatusUpdateError heals the node object after a failed status update: it creates the node when it is
// missing (together with the CSINode object it owns), otherwise it retries the status update.
// It is meant to be used as the error handler of the virtual kubelet node controller.
func (p *LiqoNodeProvider) HandleNodeStatusUpdateError(ctx context.Context, _ error) error {
	klog.Info("node setting up")

	if p.IsTerminating() {
		// This avoids the re-creation of terminated nodes.
		klog.V(4).Info("skipping: node is in terminating phase")
		return nil
	}

	newNode := p.GetNode().DeepCopy()
	newNode.ResourceVersion = ""

	oldNode, err := p.localClient.CoreV1().Nodes().Get(ctx, newNode.GetName(), metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		err := p.createNode(ctx, newNode)
		if err != nil {
			return fmt.Errorf("enforcing the Node: %w", err)
		}
		return nil
	case err != nil:
		klog.Error(err, "node error")
		return err
	}

	oldNode.Status = newNode.Status
	if _, err = p.localClient.CoreV1().Nodes().UpdateStatus(ctx, oldNode, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("updating the status of node %q: %v", oldNode.GetName(), err)
		return err
	}

	klog.V(4).Infof("Status of node %q correctly updated", oldNode.GetName())
	return nil
}

func (p *LiqoNodeProvider) createNode(ctx context.Context, node *corev1.Node) error {
	created, err := p.localClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("Failed to create node %q: %v", node.GetName(), err)
		return err
	}
	klog.Infof("Node %q correctly created", created.GetName())

	// The node may have just been created with a new UID: make sure it owns a CSINode object as well.
	if err := p.EnsureCSINode(ctx); err != nil {
		return fmt.Errorf("setting up the CSINode object for node %q: %w", created.GetName(), err)
	}

	return nil
}
