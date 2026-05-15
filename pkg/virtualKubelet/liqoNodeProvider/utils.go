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
	"encoding/json"
	gomaps "maps"
	"reflect"
	"strconv"

	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	"github.com/liqotech/liqo/pkg/utils/maps"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

func desiredVirtualNodeMetadata(
	virtualNode *offloadingv1beta1.VirtualNode,
) (labels, annotations map[string]string, taints []v1.Taint) {
	labels = gomaps.Clone(virtualNode.Spec.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	if len(virtualNode.Spec.StorageClasses) == 0 {
		labels[consts.StorageAvailableLabel] = "false"
	} else {
		labels[consts.StorageAvailableLabel] = "true"
	}

	annotations = gomaps.Clone(virtualNode.Spec.Annotations)
	if annotations == nil {
		annotations = map[string]string{}
	}

	taints = append([]v1.Taint{}, virtualNode.Spec.Taints...)
	taints = append(taints, v1.Taint{
		Key:    consts.VirtualNodeTolerationKey,
		Value:  strconv.FormatBool(true),
		Effect: v1.TaintEffectNoExecute,
	})

	return labels, annotations, taints
}

func (p *LiqoNodeProvider) applyVirtualNodeMetadata(virtualNode *offloadingv1beta1.VirtualNode) {
	labels, annotations, taints := desiredVirtualNodeMetadata(virtualNode)

	updatedLabels := maps.Sub(p.node.GetLabels(), p.lastAppliedLabels)
	p.node.Labels = maps.Merge(updatedLabels, labels)

	updatedAnnotations := maps.Sub(p.node.GetAnnotations(), p.lastAppliedAnnotations)
	p.node.Annotations = maps.Merge(updatedAnnotations, annotations)

	updatedTaints := slice.Sub(p.node.Spec.Taints, p.lastAppliedTaints)
	p.node.Spec.Taints = slice.Merge(updatedTaints, taints)

	p.lastAppliedLabels = labels
	p.lastAppliedAnnotations = annotations
	p.lastAppliedTaints = taints
}

func (p *LiqoNodeProvider) applyVirtualNodeStatus(virtualNode *offloadingv1beta1.VirtualNode) {
	if p.node.Status.Capacity == nil {
		p.node.Status.Capacity = v1.ResourceList{}
	}
	if p.node.Status.Allocatable == nil {
		p.node.Status.Allocatable = v1.ResourceList{}
	}
	for key, value := range virtualNode.Spec.ResourceQuota.Hard {
		p.node.Status.Capacity[key] = value
		p.node.Status.Allocatable[key] = value
	}

	p.node.Status.Images = append([]v1.ContainerImage{}, virtualNode.Spec.Images...)
}

func (p *LiqoNodeProvider) applyForeignCluster(foreigncluster *liqov1beta1.ForeignCluster) {
	// Only trust Networking.Enabled once the FC controller has reconciled the status.
	// Before that, the field is a zero value and indistinguishable from "networking disabled".
	if foreigncluster.Status.ObservedGeneration > 0 {
		p.networkModuleEnabled = ptr.To(fcutils.IsNetworkingModuleEnabled(foreigncluster))
	}
	p.networkReady = fcutils.IsNetworkingEstablished(foreigncluster)
}

func (p *LiqoNodeProvider) recomputeNodeState() {
	// we assume the networking module to be enabled until confirmed otherwise (p.networkModuleEnabled == false),
	// to avoid transient states where the node is ready but the network condition is not set
	// because the ForeignCluster has not been observed yet.
	networkModuleEnabled := ptr.Deref(p.networkModuleEnabled, true)

	// we have to set the network condition if we have to check the network status and the network module is enabled
	shouldSetNetworkCond := p.checkNetworkStatus && networkModuleEnabled

	// check the network status only if we have to set the network condition, otherwise consider it ready
	networkReady := p.networkReady || !shouldSetNetworkCond

	resourcesReady := areResourcesReady(p.node.Status.Allocatable)
	UpdateNodeCondition(p.node, v1.NodeReady, nodeReadyStatus(resourcesReady && networkReady))
	UpdateNodeCondition(p.node, v1.NodeMemoryPressure, nodeMemoryPressureStatus(!resourcesReady))
	UpdateNodeCondition(p.node, v1.NodeDiskPressure, nodeDiskPressureStatus(!resourcesReady))
	UpdateNodeCondition(p.node, v1.NodePIDPressure, nodePIDPressureStatus(!resourcesReady))

	if shouldSetNetworkCond {
		UpdateNodeCondition(p.node, v1.NodeNetworkUnavailable, nodeNetworkUnavailableStatus(!networkReady))
	} else {
		deleteCondition(p.node, v1.NodeNetworkUnavailable)
	}

	p.node.Status.Addresses = []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: p.nodeIP}}
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

// patchNode patches the controlled node applying the provided function.
func (p *LiqoNodeProvider) patchNode(ctx context.Context, changeFunc func(node *v1.Node) error) error {
	original, err := json.Marshal(p.node)
	if err != nil {
		klog.Error(err)
		return err
	}

	newNode := p.node.DeepCopy()
	err = changeFunc(newNode)
	if err != nil {
		klog.Error(err)
		return err
	}

	target, err := json.Marshal(newNode)
	if err != nil {
		klog.Error(err)
		return err
	}

	ops, err := jsonpatch.CreatePatch(original, target)
	if err != nil {
		klog.Error(err)
		return err
	}

	if len(ops) == 0 {
		// this avoids an empty patch of the node
		p.node = newNode
		return nil
	}

	bytes, err := json.Marshal(ops)
	if err != nil {
		klog.Error(err)
		return err
	}

	p.node, err = p.localClient.CoreV1().Nodes().Patch(ctx,
		p.node.Name, types.JSONPatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}
