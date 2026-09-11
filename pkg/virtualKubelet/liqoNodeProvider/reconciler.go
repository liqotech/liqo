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
	"errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

func (p *LiqoNodeProvider) reconcileNodeFromNode(_ watch.Event) error {
	// enforce the node to be the same as the one we are managing
	return p.updateNode()
}

func (p *LiqoNodeProvider) reconcileNodeFromVirtualNode(event watch.Event) error {
	ctx := context.Background()
	var virtualNode offloadingv1beta1.VirtualNode
	unstruct, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return errors.New("error in casting VirtualNode")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, &virtualNode); err != nil {
		klog.Error(err)
		return err
	}

	if err := p.updateFromVirtualNode(ctx, &virtualNode); err != nil {
		klog.Errorf("node update from VirtualNode %v failed for reason %v; retry...", virtualNode.Name, err)
		return err
	}
	klog.Info("node correctly updated from VirtualNode")
	return nil
}

func (p *LiqoNodeProvider) reconcileNodeFromForeignCluster(event watch.Event) error {
	var fc liqov1beta1.ForeignCluster
	unstruct, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return errors.New("error in casting ForeignCluster")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, &fc); err != nil {
		klog.Error(err)
		return err
	}

	if event.Type == watch.Deleted {
		p.updateMutex.Lock()
		defer p.updateMutex.Unlock()
		klog.Infof("foreigncluster %v deleted", fc.Name)
		p.networkReady = false
		err := p.updateNode()
		if err != nil {
			klog.Error(err)
		}
		return err
	}

	if err := p.updateFromForeignCluster(&fc); err != nil {
		klog.Errorf("node update from foreigncluster %v failed for reason %v; retry...", fc.Name, err)
		return err
	}
	return nil
}

// updateFromVirtualNode gets and updates the node status accordingly.
func (p *LiqoNodeProvider) updateFromVirtualNode(ctx context.Context,
	virtualNode *offloadingv1beta1.VirtualNode) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	lbls, annotations, taints := desiredVirtualNodeMetadata(virtualNode)

	if err := p.patchLabels(ctx, lbls); err != nil {
		klog.Error(err)
		return err
	}

	if err := p.patchAnnotations(ctx, annotations); err != nil {
		klog.Error(err)
		return err
	}

	if err := p.patchTaints(ctx, taints); err != nil {
		klog.Error(err)
		return err
	}

	p.setProviderID(virtualNode)
	p.applyVirtualNodeStatus(virtualNode)

	return p.updateNode()
}

func (p *LiqoNodeProvider) updateFromForeignCluster(foreigncluster *liqov1beta1.ForeignCluster) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	p.applyForeignCluster(foreigncluster)

	return p.updateNode()
}

func (p *LiqoNodeProvider) updateNode() error {
	p.recomputeNodeState()

	// Call change callback to notify the provider of the node update, if registered.
	if p.onNodeChangeCallback != nil {
		p.onNodeChangeCallback(p.node.DeepCopy())
	}

	return nil
}

// areResourcesReady returns true if both cpu and memory are more than zero.
func areResourcesReady(allocatable v1.ResourceList) bool {
	if allocatable == nil {
		return false
	}
	cpu := allocatable.Cpu()
	if cpu == nil || cpu.CmpInt64(0) <= 0 {
		return false
	}
	memory := allocatable.Memory()
	if memory == nil || memory.CmpInt64(0) <= 0 {
		return false
	}
	return true
}
