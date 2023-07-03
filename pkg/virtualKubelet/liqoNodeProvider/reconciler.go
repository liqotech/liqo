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

package liqonodeprovider

import (
	"context"
	"errors"
	"reflect"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/maps"
)

func (p *LiqoNodeProvider) reconcileNodeFromVirtualNode(event watch.Event) error {
	ctx := context.Background()
	var virtualNode virtualkubeletv1alpha1.VirtualNode
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

func (p *LiqoNodeProvider) reconcileNodeFromTep(event watch.Event) error {
	var tep netv1alpha1.TunnelEndpoint
	unstruct, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return errors.New("error in casting tunnel endpoint: recreate watcher")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, &tep); err != nil {
		klog.Error(err)
		return err
	}
	if event.Type == watch.Deleted {
		p.updateMutex.Lock()
		defer p.updateMutex.Unlock()
		klog.Infof("tunnelEndpoint %v deleted", tep.Name)
		p.networkReady = false
		err := p.updateNode()
		if err != nil {
			klog.Error(err)
		}
		return err
	}

	if err := p.updateFromTep(&tep); err != nil {
		klog.Errorf("node update from tunnelEndpoint %v failed for reason %v; retry...", tep.Name, err)
		return err
	}
	klog.Info("correctly set pod CIDR from tunnel endpoint")
	return nil
}

// updateFromVirtualNode gets and updates the node status accordingly.
func (p *LiqoNodeProvider) updateFromVirtualNode(ctx context.Context,
	virtualNode *virtualkubeletv1alpha1.VirtualNode) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	lbls := virtualNode.Spec.Labels
	if lbls == nil {
		lbls = map[string]string{}
	}
	if len(virtualNode.Spec.StorageClasses) == 0 {
		lbls[consts.StorageAvailableLabel] = "false"
	} else {
		lbls[consts.StorageAvailableLabel] = "true"
	}

	if err := p.patchLabels(ctx, lbls); err != nil {
		klog.Error(err)
		return err
	}

	if p.node.Status.Capacity == nil {
		p.node.Status.Capacity = v1.ResourceList{}
	}
	if p.node.Status.Allocatable == nil {
		p.node.Status.Allocatable = v1.ResourceList{}
	}
	for k, v := range virtualNode.Spec.ResourceQuota.Hard {
		p.node.Status.Capacity[k] = v
		p.node.Status.Allocatable[k] = v
	}

	p.node.Status.Images = []v1.ContainerImage{}
	p.node.Status.Images = append(p.node.Status.Images, virtualNode.Spec.Images...)

	return p.updateNode()
}

func (p *LiqoNodeProvider) updateFromTep(tep *netv1alpha1.TunnelEndpoint) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	// if tep is not connected yet, return
	if tep.Status.Connection.Status != netv1alpha1.Connected {
		p.networkReady = false
		return p.updateNode()
	}
	p.networkReady = true
	return p.updateNode()
}

func (p *LiqoNodeProvider) updateNode() error {
	resourcesReady := areResourcesReady(p.node.Status.Allocatable)
	networkReady := p.networkReady || !p.checkNetworkStatus

	UpdateNodeCondition(p.node, v1.NodeReady, nodeReadyStatus(resourcesReady && networkReady))
	UpdateNodeCondition(p.node, v1.NodeMemoryPressure, nodeMemoryPressureStatus(!resourcesReady))
	UpdateNodeCondition(p.node, v1.NodeDiskPressure, nodeDiskPressureStatus(!resourcesReady))
	UpdateNodeCondition(p.node, v1.NodePIDPressure, nodePIDPressureStatus(!resourcesReady))
	if p.checkNetworkStatus {
		UpdateNodeCondition(p.node, v1.NodeNetworkUnavailable, nodeNetworkUnavailableStatus(!networkReady))
	}

	p.onNodeChangeCallback(p.node.DeepCopy())
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

func (p *LiqoNodeProvider) patchLabels(ctx context.Context, labels map[string]string) error {
	if reflect.DeepEqual(labels, p.lastAppliedLabels) {
		return nil
	}
	if labels == nil {
		labels = map[string]string{}
	}

	if err := p.patchNode(ctx, func(node *v1.Node) error {
		nodeLabels := node.GetLabels()
		nodeLabels = maps.Sub(nodeLabels, p.lastAppliedLabels)
		nodeLabels = maps.Merge(nodeLabels, labels)
		node.Labels = nodeLabels
		return nil
	}); err != nil {
		klog.Error(err)
		return err
	}

	p.lastAppliedLabels = labels
	return nil
}
