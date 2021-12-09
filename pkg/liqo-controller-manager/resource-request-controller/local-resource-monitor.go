// Copyright 2019-2021 The Liqo Authors
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

package resourcerequestoperator

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
	"github.com/liqotech/liqo/pkg/utils/pod"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// LocalResourceMonitor is an object that keeps track of the cluster's resources.
type LocalResourceMonitor struct {
	allocatable    corev1.ResourceList
	resourcePodMap map[string]corev1.ResourceList
	nodeMutex      sync.RWMutex
	podMutex       sync.RWMutex
	notifier       ResourceUpdateNotifier
}

// PodTransition represents a podReady condition possible transitions.
type PodTransition uint8

const (
	// PendingToReady represents a transition from PodReady status = false to PodReady status = true.
	PendingToReady PodTransition = iota
	// ReadyToReady represents no change in PodReady status when status = true.
	ReadyToReady
	// ReadyToPending represents a transition from PodReady status = true to PodReady status = false.
	ReadyToPending
	// PendingToPending represents no change in PodReady status when status = false.
	PendingToPending
)

// NewLocalMonitor creates a new LocalResourceMonitor.
func NewLocalMonitor(ctx context.Context, clientset kubernetes.Interface,
	resyncPeriod time.Duration) *LocalResourceMonitor {
	nodeFactory := informers.NewSharedInformerFactoryWithOptions(
		clientset, resyncPeriod, informers.WithTweakListOptions(noVirtualNodesFilter),
	)
	nodeInformer := nodeFactory.Core().V1().Nodes().Informer()
	podFactory := informers.NewSharedInformerFactoryWithOptions(
		clientset, resyncPeriod, informers.WithTweakListOptions(noShadowPodsFilter),
	)
	podInformer := podFactory.Core().V1().Pods().Informer()

	lrm := LocalResourceMonitor{
		allocatable:    corev1.ResourceList{},
		resourcePodMap: map[string]corev1.ResourceList{},
	}

	nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    lrm.onNodeAdd,
		UpdateFunc: lrm.onNodeUpdate,
		DeleteFunc: lrm.onNodeDelete,
	})
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    lrm.onPodAdd,
		UpdateFunc: lrm.onPodUpdate,
		DeleteFunc: lrm.onPodDelete,
	})

	nodeFactory.Start(ctx.Done())
	nodeFactory.WaitForCacheSync(ctx.Done())
	podFactory.Start(ctx.Done())
	podFactory.WaitForCacheSync(ctx.Done())

	return &lrm
}

// Register sets an update notifier.
func (m *LocalResourceMonitor) Register(notifier ResourceUpdateNotifier) {
	m.notifier = notifier
}

// react to a Node Creation/First informer run.
func (m *LocalResourceMonitor) onNodeAdd(obj interface{}) {
	node := obj.(*corev1.Node)
	if utils.IsNodeReady(node) {
		klog.V(4).Infof("Adding Node %s", node.Name)
		toAdd := &node.Status.Allocatable
		currentResources := m.readClusterResources()
		addResources(currentResources, *toAdd)
		m.writeClusterResources(currentResources)
	}
}

// react to a Node Update.
func (m *LocalResourceMonitor) onNodeUpdate(oldObj, newObj interface{}) {
	oldNode := oldObj.(*corev1.Node)
	newNode := newObj.(*corev1.Node)
	oldNodeResources := oldNode.Status.Allocatable
	newNodeResources := newNode.Status.Allocatable
	currentResources := m.readClusterResources()
	klog.V(4).Infof("Updating Node %s", oldNode.Name)
	if utils.IsNodeReady(newNode) {
		// node was already Ready, update with possible new resources.
		if utils.IsNodeReady(oldNode) {
			updateResources(currentResources, oldNodeResources, newNodeResources)
			// node is starting, add all its resources.
		} else {
			addResources(currentResources, newNodeResources)
		}
		// node is terminating or stopping, delete all its resources.
	} else if utils.IsNodeReady(oldNode) && !utils.IsNodeReady(newNode) {
		subResources(currentResources, oldNodeResources)
	}
	m.writeClusterResources(currentResources)
}

// react to a Node Delete.
func (m *LocalResourceMonitor) onNodeDelete(obj interface{}) {
	node := obj.(*corev1.Node)
	toDelete := &node.Status.Allocatable
	currentResources := m.readClusterResources()
	if utils.IsNodeReady(node) {
		klog.V(4).Infof("Deleting Node %s", node.Name)
		subResources(currentResources, *toDelete)
		m.writeClusterResources(currentResources)
	}
}

func (m *LocalResourceMonitor) onPodAdd(obj interface{}) {
	podAdded := obj.(*corev1.Pod)
	klog.V(4).Infof("OnPodAdd: Add for pod %s:%s", podAdded.Namespace, podAdded.Name)
	// ignore all shadow pods because of ignoring all virtual Nodes
	if ready, _ := pod.IsPodReady(podAdded); ready {
		podResources := extractPodResources(podAdded)
		currentResources := m.readClusterResources()
		// subtract the pod resource from cluster resources. This action is done for all pods to extract actual available resources.
		subResources(currentResources, podResources)
		m.writeClusterResources(currentResources)
		if clusterID := podAdded.Labels[forge.LiqoOriginClusterIDKey]; clusterID != "" {
			klog.V(4).Infof("OnPodAdd: Pod %s:%s passed ClusterID check. ClusterID = %s", podAdded.Namespace, podAdded.Name, clusterID)
			currentPodsResources := m.readPodResources(clusterID)
			// add the resource of this pod in the map clusterID => resources to be used in ReadResources() function.
			// this action is done to correct the computation not considering pod offloaded by the cluster with this ClusterID
			addResources(currentPodsResources, podResources)
			m.writePodResources(clusterID, currentPodsResources)
		}
	}
}

func (m *LocalResourceMonitor) onPodUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)
	klog.V(4).Infof("OnPodUpdate: Update for pod %s:%s", newPod.Namespace, newPod.Name)
	newResources := extractPodResources(newPod)
	oldResources := extractPodResources(oldPod)
	currentResources := m.readClusterResources()
	clusterID := newPod.Labels[forge.LiqoOriginClusterIDKey]
	// empty if clusterID has not a valid value.
	currentPodsResources := m.readPodResources(clusterID)

	switch getPodTransitionState(oldPod, newPod) {
	// pod is becoming Ready, same of onPodAdd case.
	case PendingToReady:
		subResources(currentResources, newResources)
		if clusterID != "" {
			klog.V(4).Infof("OnPodUpdate: Pod %s:%s passed ClusterID check. ClusterID = %s", newPod.Namespace, newPod.Name, clusterID)
			// add the resource of this pod in the map clusterID => resources to be used in ReadResources() function.
			// this action is done to correct the computation not considering pod offloaded by the cluster with this ClusterID
			addResources(currentPodsResources, newResources)
		}
	// pod is no more Ready, same of onDeletePod case.
	case ReadyToPending:
		addResources(currentResources, newResources)
		if clusterID != "" {
			klog.V(4).Infof("OnPodUpdate: Pod %s:%s passed ClusterID check. ClusterID = %s", newPod.Namespace, newPod.Name, clusterID)
			// sub the resource of this pod in the map clusterID => resources to be used in ReadResources() function.
			// this action is done to correct the computation not considering pod offloaded by the cluster with this ClusterID
			subResources(currentPodsResources, oldResources)
		}
	// pod resources request are immutable. See the doc https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	case ReadyToReady:
		return
	case PendingToPending:
		return
	}
	m.writeClusterResources(currentResources)
	m.writePodResources(clusterID, currentPodsResources)
}

func (m *LocalResourceMonitor) onPodDelete(obj interface{}) {
	podDeleted := obj.(*corev1.Pod)
	klog.V(4).Infof("OnPodDelete: Delete for pod %s:%s", podDeleted.Namespace, podDeleted.Name)
	// ignore all shadow pods because of ignoring all virtual Nodes
	if ready, _ := pod.IsPodReady(podDeleted); ready {
		podResources := extractPodResources(podDeleted)
		currentResources := m.readClusterResources()
		// Resources used by the pod will become available again so add them to the total allocatable ones.
		addResources(currentResources, podResources)
		m.writeClusterResources(currentResources)
		if clusterID := podDeleted.Labels[forge.LiqoOriginClusterIDKey]; clusterID != "" {
			klog.V(4).Infof("OnPodDelete: Pod %s:%s passed ClusterID check. ClusterID = %s", podDeleted.Namespace, podDeleted.Name, clusterID)
			currentPodsResources := m.readPodResources(clusterID)
			subResources(currentPodsResources, podResources)
			m.writePodResources(clusterID, currentPodsResources)
		}
	}
}

// write nodes resources in thread safe mode.
func (m *LocalResourceMonitor) writeClusterResources(newResources corev1.ResourceList) {
	if !liqoerrors.Must(checkSign(newResources)) {
		setZero(&newResources)
	}
	m.nodeMutex.Lock()
	m.allocatable = newResources.DeepCopy()
	m.nodeMutex.Unlock()
	m.notifyOrWarn()
}

// write pods resources in thread safe mode.
func (m *LocalResourceMonitor) writePodResources(clusterID string, newResources corev1.ResourceList) {
	if clusterID == "" {
		return
	}
	if !liqoerrors.Must(checkSign(newResources)) {
		setZero(&newResources)
	}
	m.podMutex.Lock()
	m.resourcePodMap[clusterID] = newResources.DeepCopy()
	m.podMutex.Unlock()
	m.notifyOrWarn()
}

func (m *LocalResourceMonitor) notifyOrWarn() {
	if m.notifier == nil {
		klog.Warning("No notifier is configured, an update will be lost")
	} else {
		m.notifier.NotifyChange()
	}
}

// ReadResources returns the resources available in the cluster (total minus used), multiplied by resourceSharingPercentage.
func (m *LocalResourceMonitor) ReadResources(clusterID string) corev1.ResourceList {
	toRead := m.readClusterResources()
	podsResources := m.readPodResources(clusterID)
	addResources(toRead, podsResources)
	return toRead
}

// RemoveClusterID removes a clusterID from all broadcaster internal structures
// it is useful when a particular foreign cluster has no more peering and its ResourceRequest has been deleted.
func (m *LocalResourceMonitor) RemoveClusterID(clusterID string) {
	m.podMutex.Lock()
	defer m.podMutex.Unlock()
	delete(m.resourcePodMap, clusterID)
}

// readClusterResources returns the total resources in the cluster.
// It performs thread-safe access to allocatable.
func (m *LocalResourceMonitor) readClusterResources() corev1.ResourceList {
	m.nodeMutex.RLock()
	defer m.nodeMutex.RUnlock()
	return m.allocatable.DeepCopy()
}

// readClusterResources returns the resources used by pods in the cluster.
// It performs thread-safe access to resourcePodMap.
func (m *LocalResourceMonitor) readPodResources(clusterID string) corev1.ResourceList {
	m.podMutex.RLock()
	defer m.podMutex.RUnlock()
	if toRead, exists := m.resourcePodMap[clusterID]; exists {
		return toRead.DeepCopy()
	}
	return corev1.ResourceList{}
}

func setZero(resources *corev1.ResourceList) {
	for resourceName, value := range *resources {
		value.Set(0)
		(*resources)[resourceName] = value
	}
}

// addResources is a utility function to add resources.
func addResources(currentResources, toAdd corev1.ResourceList) {
	for resourceName, quantity := range toAdd {
		if value, exists := currentResources[resourceName]; exists {
			value.Add(quantity)
			currentResources[resourceName] = value
		} else {
			currentResources[resourceName] = quantity
		}
	}
}

// subResources is an utility function to subtract resources.
func subResources(currentResources, toSub corev1.ResourceList) {
	for resourceName, quantity := range toSub {
		if value, exists := currentResources[resourceName]; exists {
			value.Sub(quantity)
			currentResources[resourceName] = value
		}
	}
}

// updateResources is a utility function to update resources.
func updateResources(currentResources, oldResources, newResources corev1.ResourceList) {
	for resourceName, quantity := range newResources {
		if oldQuantity, exists := oldResources[resourceName]; exists {
			value := currentResources[resourceName]
			quantityToUpdate := resource.NewQuantity(quantity.Value()-oldQuantity.Value(),
				quantity.Format)
			value.Add(*quantityToUpdate)
			currentResources[resourceName] = value
		} else {
			currentResources[resourceName] = quantity
		}
	}
}

func extractPodResources(podToExtract *corev1.Pod) corev1.ResourceList {
	resourcesToExtract, _ := resourcehelper.PodRequestsAndLimits(podToExtract)
	return resourcesToExtract
}

func checkSign(currentResources corev1.ResourceList) error {
	for resourceName, value := range currentResources {
		if value.Sign() == -1 {
			return fmt.Errorf("resource %s has a negative value: %v", resourceName, value.String())
		}
	}
	return nil
}

func getPodTransitionState(oldPod, newPod *corev1.Pod) PodTransition {
	newOk, _ := pod.IsPodReady(newPod)
	oldOk, _ := pod.IsPodReady(oldPod)

	if newOk && oldOk {
		return ReadyToReady
	}

	if newOk && !oldOk {
		return PendingToReady
	}

	if !newOk && oldOk {
		return ReadyToPending
	}

	return PendingToPending
}

// this function is used to filter and ignore virtual nodes at informer level.
func noVirtualNodesFilter(options *metav1.ListOptions) {
	var values []string
	values = append(values, consts.TypeNode)
	req, err := labels.NewRequirement(consts.TypeLabel, selection.NotEquals, values)
	if err != nil {
		return
	}
	options.LabelSelector = labels.NewSelector().Add(*req).String()
}

// this function is used to filter and ignore shadow pods at informer level.
func noShadowPodsFilter(options *metav1.ListOptions) {
	var values []string
	values = append(values, consts.LocalPodLabelValue)
	req, err := labels.NewRequirement(consts.LocalPodLabelKey, selection.NotEquals, values)
	if err != nil {
		return
	}
	options.LabelSelector = labels.NewSelector().Add(*req).String()
}

func isShadowPod(podToCheck *corev1.Pod) bool {
	if shadowLabel, exists := podToCheck.Labels[consts.LocalPodLabelKey]; exists {
		if shadowLabel == consts.LocalPodLabelValue {
			return true
		}
	}
	return false
}
