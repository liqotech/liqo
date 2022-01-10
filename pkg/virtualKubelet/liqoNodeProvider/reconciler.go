// Copyright 2019-2022 The Liqo Authors
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
	"errors"
	"reflect"

	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
)

func isResourceOfferTerminating(resourceOffer *sharingv1alpha1.ResourceOffer) bool {
	hasTimestamp := !resourceOffer.DeletionTimestamp.IsZero()
	desiredDelete := !resourceOffer.Spec.WithdrawalTimestamp.IsZero()
	return hasTimestamp || desiredDelete
}

// The reconciliation function; every time this function is called,
// the node status is updated by means of r.updateFromResourceOffer.
func (p *LiqoNodeProvider) reconcileNodeFromResourceOffer(event watch.Event) error {
	var resourceOffer sharingv1alpha1.ResourceOffer
	unstruct, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return errors.New("error in casting ResourceOffer")
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstruct.Object, &resourceOffer); err != nil {
		klog.Error(err)
		return err
	}

	if event.Type == watch.Deleted || isResourceOfferTerminating(&resourceOffer) {
		p.updateMutex.Lock()
		defer p.updateMutex.Unlock()
		klog.Infof("resourceOffer %v is going to be deleted... set node status not ready", resourceOffer.Name)
		p.terminating = true
		for i, condition := range p.node.Status.Conditions {
			switch condition.Type {
			case v1.NodeReady:
				p.node.Status.Conditions[i].Status = v1.ConditionFalse
			case v1.NodeMemoryPressure:
				p.node.Status.Conditions[i].Status = v1.ConditionTrue
			default:
			}
		}
		p.node.Status.Allocatable = v1.ResourceList{}
		p.node.Status.Capacity = v1.ResourceList{}
		p.onNodeChangeCallback(p.node.DeepCopy())

		if err := p.handleResourceOfferDelete(&resourceOffer); err != nil {
			klog.Errorf("something went wrong during resourceOffer deletion - %v", err)
			return err
		}
		return nil
	}

	if err := p.ensureFinalizer(&resourceOffer, func() bool {
		return !controllerutil.ContainsFinalizer(&resourceOffer, consts.NodeFinalizer)
	}, controllerutil.AddFinalizer); err != nil {
		klog.Error(err)
		return err
	}

	if err := p.updateFromResourceOffer(&resourceOffer); err != nil {
		klog.Errorf("node update from resourceOffer %v failed for reason %v; retry...", resourceOffer.Name, err)
		return err
	}
	klog.Info("node correctly updated from resourceOffer")
	return nil
}

// ensureFinalizer ensures the finalizer status. The patch will be applied if the provided check function
// returns true, and it will build applying the provided changeFinalizer function.
func (p *LiqoNodeProvider) ensureFinalizer(resourceOffer *sharingv1alpha1.ResourceOffer,
	check func() bool, changeFinalizer func(client.Object, string)) error {
	if check() {
		original, err := json.Marshal(resourceOffer)
		if err != nil {
			klog.Error(err)
			return err
		}

		changeFinalizer(resourceOffer, consts.NodeFinalizer)

		target, err := json.Marshal(resourceOffer)
		if err != nil {
			klog.Error(err)
			return err
		}

		ops, err := jsonpatch.CreatePatch(original, target)
		if err != nil {
			klog.Error(err)
			return err
		}

		bytes, err := json.Marshal(ops)
		if err != nil {
			klog.Error(err)
			return err
		}

		_, err = p.dynClient.Resource(sharingv1alpha1.GroupVersion.WithResource("resourceoffers")).
			Namespace(resourceOffer.GetNamespace()).
			Patch(context.TODO(), resourceOffer.GetName(), types.JSONPatchType, bytes, metav1.PatchOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
	}
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

// updateFromResourceOffer gets and updates the node status accordingly.
func (p *LiqoNodeProvider) updateFromResourceOffer(resourceOffer *sharingv1alpha1.ResourceOffer) error {
	p.updateMutex.Lock()
	defer p.updateMutex.Unlock()

	lbls := resourceOffer.Spec.Labels
	if lbls == nil {
		lbls = map[string]string{}
	}
	if len(resourceOffer.Spec.StorageClasses) == 0 {
		lbls[consts.StorageAvailableLabel] = "false"
	} else {
		lbls[consts.StorageAvailableLabel] = "true"
	}

	if err := p.patchLabels(lbls); err != nil {
		klog.Error(err)
		return err
	}

	if p.node.Status.Capacity == nil {
		p.node.Status.Capacity = v1.ResourceList{}
	}
	if p.node.Status.Allocatable == nil {
		p.node.Status.Allocatable = v1.ResourceList{}
	}
	for k, v := range resourceOffer.Spec.ResourceQuota.Hard {
		p.node.Status.Capacity[k] = v
		p.node.Status.Allocatable[k] = v
	}

	p.node.Status.Images = []v1.ContainerImage{}
	p.node.Status.Images = append(p.node.Status.Images, resourceOffer.Spec.Images...)

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

	UpdateNodeCondition(p.node, v1.NodeReady, nodeReadyStatus(resourcesReady && p.networkReady))
	UpdateNodeCondition(p.node, v1.NodeMemoryPressure, nodeMemoryPressureStatus(!resourcesReady))
	UpdateNodeCondition(p.node, v1.NodeDiskPressure, nodeDiskPressureStatus(!resourcesReady))
	UpdateNodeCondition(p.node, v1.NodePIDPressure, nodePIDPressureStatus(!resourcesReady))
	UpdateNodeCondition(p.node, v1.NodeNetworkUnavailable, nodeNetworkUnavailableStatus(!p.networkReady))

	p.onNodeChangeCallback(p.node.DeepCopy())
	return nil
}

func (p *LiqoNodeProvider) handleResourceOfferDelete(resourceOffer *sharingv1alpha1.ResourceOffer) error {
	ctx := context.TODO()

	if err := client.IgnoreNotFound(p.cordonNode(ctx)); err != nil {
		klog.Errorf("error cordoning node: %v", err)
		return err
	}

	if err := client.IgnoreNotFound(p.drainNode(ctx)); err != nil {
		klog.Errorf("error draining node: %v", err)
		return err
	}

	// delete the node
	if err := client.IgnoreNotFound(p.localClient.CoreV1().Nodes().Delete(ctx, p.node.GetName(), metav1.DeleteOptions{})); err != nil {
		klog.Errorf("error deleting node: %v", err)
		return err
	}

	// remove the finalizer
	if err := p.ensureFinalizer(resourceOffer, func() bool {
		return controllerutil.ContainsFinalizer(resourceOffer, consts.NodeFinalizer)
	}, controllerutil.RemoveFinalizer); err != nil {
		klog.Errorf("error removing finalizer from resource offer %v/%v: %v", resourceOffer.GetNamespace(), resourceOffer.GetName(), err)
		return err
	}

	return nil
}

func (p *LiqoNodeProvider) patchLabels(labels map[string]string) error {
	if reflect.DeepEqual(labels, p.lastAppliedLabels) {
		return nil
	}
	if labels == nil {
		labels = map[string]string{}
	}

	if err := p.patchNode(func(node *v1.Node) error {
		nodeLabels := node.GetLabels()
		nodeLabels = utils.SubMaps(nodeLabels, p.lastAppliedLabels)
		nodeLabels = utils.MergeMaps(nodeLabels, labels)
		node.Labels = nodeLabels
		return nil
	}); err != nil {
		klog.Error(err)
		return err
	}

	p.lastAppliedLabels = labels
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
