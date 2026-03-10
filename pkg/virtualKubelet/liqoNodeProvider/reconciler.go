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
	"reflect"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/pkg/utils/maps"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

func (p *LiqoNodeProvider) reconcileNodeFromNode(_ watch.Event) error {
	// enforce the node to be the same as the one we are managing
	klog.V(4).Info("reconciling node from local node event")
	return p.updateNode()
}

func (p *LiqoNodeProvider) reconcileNodeFromVirtualNode(event watch.Event) error {
	klog.V(4).Info("reconciling node from virtual node event")
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
	return nil
}

func (p *LiqoNodeProvider) reconcileNodeFromForeignCluster(event watch.Event) error {
	klog.V(4).Info("reconciling node from foreigncluster event")
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

func (p *LiqoNodeProvider) reconcileNodeFromRemoteNode(event watch.Event) error {
	klog.V(4).Info("reconciling node from remote node event")
	var remoteNode v1.Node
	unstruct, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return errors.New("error in casting Remote Node")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, &remoteNode); err != nil {
		klog.Error(err)
		return err
	}

	if err := p.updateFromRemoteNode(&remoteNode); err != nil {
		klog.Errorf("node update from remote node %v failed for reason %v; retry...", remoteNode.Name, err)
		return err
	}
	return nil
}

// updateFromVirtualNode gets and updates the node status accordingly.
func (p *LiqoNodeProvider) updateFromVirtualNode(ctx context.Context,
	virtualNode *offloadingv1beta1.VirtualNode) error {
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

	annotations := virtualNode.Spec.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}
	if err := p.patchAnnotations(ctx, annotations); err != nil {
		klog.Error(err)
		return err
	}

	taints := virtualNode.Spec.Taints
	if taints == nil {
		taints = []v1.Taint{}
	}
	if err := p.patchTaints(ctx, taints); err != nil {
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

func (p *LiqoNodeProvider) updateFromForeignCluster(foreigncluster *liqov1beta1.ForeignCluster) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	// Only trust Networking.Enabled once the FC controller has reconciled the status.
	// Before that, the field is a zero value and indistinguishable from "networking disabled".
	if foreigncluster.Status.ObservedGeneration > 0 {
		p.networkModuleEnabled = ptr.To(fcutils.IsNetworkingModuleEnabled(foreigncluster))
	}
	p.networkReady = fcutils.IsNetworkingEstablished(foreigncluster)

	return p.updateNode()
}

func (p *LiqoNodeProvider) updateFromRemoteNode(remoteNode *v1.Node) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	p.remoteNodeStatus = remoteNode.Status

	return p.updateNode()
}

func (p *LiqoNodeProvider) updateNode() error {
	// we assume the networking module to be enabled until confirmed otherwise (p.networkModuleEnabled == false),
	// to avoid transient states where the node is ready but the network condition is not set
	// because the ForeignCluster has not been observed yet.
	networkModuleEnabled := ptr.Deref(p.networkModuleEnabled, true)

	// we have to set the network condition if we have to check the network status and the network module is enabled
	shouldSetNetworkCond := p.checkNetworkStatus && networkModuleEnabled

	// check the network status only if we have to set the network condition, otherwise consider it ready
	networkReady := p.networkReady || !shouldSetNetworkCond

	if p.watchRemoteNode {
		conditionStatus := func(condType v1.NodeConditionType) func() (v1.ConditionStatus, string, string) {
			cond, _ := lookupConditionOrCreateUnknown(p.remoteNodeStatus.Conditions, condType)
			return func() (status v1.ConditionStatus, reason, message string) {
				return cond.Status, cond.Reason, cond.Message
			}
		}

		remoteReadyStatus, _, _ := conditionStatus(v1.NodeReady)()
		UpdateNodeCondition(p.node, v1.NodeReady, nodeReadyStatus(remoteReadyStatus == v1.ConditionTrue && networkReady))
		UpdateNodeCondition(p.node, v1.NodeMemoryPressure, conditionStatus(v1.NodeMemoryPressure))
		UpdateNodeCondition(p.node, v1.NodeDiskPressure, conditionStatus(v1.NodeDiskPressure))
		UpdateNodeCondition(p.node, v1.NodePIDPressure, conditionStatus(v1.NodePIDPressure))
	} else {
		// Legacy resource setter.
		resourcesReady := areResourcesReady(p.node.Status.Allocatable)
		UpdateNodeCondition(p.node, v1.NodeReady, nodeReadyStatus(resourcesReady && networkReady))
		UpdateNodeCondition(p.node, v1.NodeMemoryPressure, nodeMemoryPressureStatus(!resourcesReady))
		UpdateNodeCondition(p.node, v1.NodeDiskPressure, nodeDiskPressureStatus(!resourcesReady))
		UpdateNodeCondition(p.node, v1.NodePIDPressure, nodePIDPressureStatus(!resourcesReady))
	}

	if shouldSetNetworkCond {
		UpdateNodeCondition(p.node, v1.NodeNetworkUnavailable, nodeNetworkUnavailableStatus(!networkReady))
	} else {
		deleteCondition(p.node, v1.NodeNetworkUnavailable)
	}

	p.node.Status.Addresses = []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: p.nodeIP}}
	if p.watchRemoteNode {
		p.node.Status.NodeInfo = p.remoteNodeStatus.NodeInfo
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

func (p *LiqoNodeProvider) patchAnnotations(ctx context.Context, annotations map[string]string) error {
	if reflect.DeepEqual(annotations, p.lastAppliedAnnotations) {
		return nil
	}
	if annotations == nil {
		annotations = map[string]string{}
	}

	if err := p.patchNode(ctx, func(node *v1.Node) error {
		nodeAnnotations := node.GetAnnotations()
		nodeAnnotations = maps.Sub(nodeAnnotations, p.lastAppliedAnnotations)
		nodeAnnotations = maps.Merge(nodeAnnotations, annotations)
		node.Annotations = nodeAnnotations
		return nil
	}); err != nil {
		klog.Error(err)
		return err
	}

	p.lastAppliedAnnotations = annotations
	return nil
}

func (p *LiqoNodeProvider) patchTaints(ctx context.Context, taints []v1.Taint) error {
	if taints == nil {
		taints = []v1.Taint{}
	}
	taints = append(taints, v1.Taint{
		Key:    consts.VirtualNodeTolerationKey,
		Value:  strconv.FormatBool(true),
		Effect: v1.TaintEffectNoExecute,
	})

	if reflect.DeepEqual(taints, p.lastAppliedTaints) {
		return nil
	}
	if taints == nil {
		taints = []v1.Taint{}
	}

	if err := p.patchNode(ctx, func(node *v1.Node) error {
		nodeTaints := node.Spec.Taints
		nodeTaints = slice.Sub(nodeTaints, p.lastAppliedTaints)
		nodeTaints = slice.Merge(nodeTaints, taints)
		node.Spec.Taints = nodeTaints
		return nil
	}); err != nil {
		klog.Error(err)
		return err
	}

	p.lastAppliedTaints = taints
	return nil
}
