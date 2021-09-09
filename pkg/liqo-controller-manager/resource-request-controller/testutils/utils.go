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

package testutils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// DefaultScalePercentage defines the amount of scaled resources to be computed in resourceOffers.
const DefaultScalePercentage = 50

// CreateNewNode forges a new node with name *nodeName* and creates it using the *clientset* client.
func CreateNewNode(ctx context.Context, nodeName string, virtual bool, clientset kubernetes.Interface) (*corev1.Node, error) {
	resources := corev1.ResourceList{}
	resources[corev1.ResourceCPU] = *resource.NewScaledQuantity(5000, resource.Milli)
	resources[corev1.ResourceMemory] = *resource.NewScaledQuantity(5, resource.Mega)
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
	if virtual {
		node.Labels = map[string]string{
			consts.TypeLabel: consts.TypeNode,
		}
	}
	node.Status = corev1.NodeStatus{
		Capacity:    resources,
		Allocatable: resources,
		Conditions: []corev1.NodeCondition{
			0: {
				Type:   corev1.NodeReady,
				Status: corev1.ConditionTrue,
			},
		},
	}
	node, err := clientset.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return node, nil
}

// CreateNewPod forges a new pod with name *podName* and creates it using the *clientset* client.
func CreateNewPod(ctx context.Context, podName, clusterID string, shadow bool, clientset kubernetes.Interface) (*corev1.Pod, error) {
	resources := corev1.ResourceList{}
	resources[corev1.ResourceCPU] = *resource.NewQuantity(1, resource.DecimalSI)
	resources[corev1.ResourceMemory] = *resource.NewQuantity(50000, resource.DecimalSI)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default",
			Labels:    map[string]string{},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				0: {
					Name: "test-container1",
					Resources: corev1.ResourceRequirements{
						Limits:   resources,
						Requests: resources,
					},
					Image: "nginx",
				},
			},
		},
	}
	if clusterID != "" {
		pod.Labels[forge.LiqoOutgoingKey] = "test"
		pod.Labels[forge.LiqoOriginClusterID] = clusterID
	}
	if shadow {
		pod.Labels[consts.LocalPodLabelKey] = consts.LocalPodLabelValue
	}
	pod, err := clientset.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	// set Status Ready
	pod.Status = corev1.PodStatus{
		Conditions: []corev1.PodCondition{
			0: {
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			},
		},
	}
	pod, err = clientset.CoreV1().Pods("default").UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return pod, nil
}

// SetPodReadyStatus enforces a status ready/not ready to a pod passed as the *pod* parameter. The readiness/not readiness is
// enforced by the *status* bool (true is ready, false is not ready).
func SetPodReadyStatus(ctx context.Context, pod *corev1.Pod, status bool, clientset kubernetes.Interface) (*corev1.Pod, error) {
	for key, value := range pod.Status.Conditions {
		if value.Type == corev1.PodReady {
			if status {
				pod.Status.Conditions[key].Status = corev1.ConditionTrue
			} else {
				pod.Status.Conditions[key].Status = corev1.ConditionFalse
			}
		}
	}
	pod, err := clientset.CoreV1().Pods("default").UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return pod, err
}

// SetNodeReadyStatus enforces a status ready/not ready to a node passed as the *node* parameter. The readiness/not readiness is
// enforced by the *status* bool (true is ready, false is not ready).
func SetNodeReadyStatus(ctx context.Context, node *corev1.Node, status bool, clientset kubernetes.Interface) (*corev1.Node, error) {
	for key, value := range node.Status.Conditions {
		if value.Type == corev1.NodeReady {
			if status {
				node.Status.Conditions[key].Status = corev1.ConditionTrue
			} else {
				node.Status.Conditions[key].Status = corev1.ConditionFalse
			}
		}
	}
	node, err := clientset.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return node, nil
}

// CheckResourceOfferUpdate returns false if the (1) resource offer does not exist or the get returns an error (2) if the scaled quantity
// available in the resourceOffer is not equal to the one present in the cluster. It returns true otherwise.
func CheckResourceOfferUpdate(ctx context.Context, offerPrefix, homeClusterID, resourcesNamespace string,
	nodeResources, podResources []corev1.ResourceList, k8sClient client.Client) bool {
	offer := &sharingv1alpha1.ResourceOffer{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      offerPrefix + homeClusterID,
		Namespace: resourcesNamespace,
	}, offer)
	if err != nil {
		return false
	}
	offerResources := offer.Spec.ResourceQuota.Hard
	testList := corev1.ResourceList{}
	for _, nodeResource := range nodeResources {
		for resourceName, quantity := range nodeResource {
			toAdd := testList[resourceName].DeepCopy()
			toAdd.Add(quantity)
			testList[resourceName] = toAdd.DeepCopy()
		}
	}

	for _, podResource := range podResources {
		for resourceName, quantity := range podResource {
			toSub := testList[resourceName].DeepCopy()
			toSub.Sub(quantity)
			testList[resourceName] = toSub.DeepCopy()
		}
	}

	for resourceName, quantity := range offerResources {
		toCheck := testList[resourceName].DeepCopy()
		Scale(DefaultScalePercentage, resourceName, &toCheck)
		if quantity.Cmp(toCheck) != 0 {
			return false
		}
	}
	return true
}

// Scale sets a *percentage* for a given *quantity* of a specific *resourceName*.
func Scale(percentage int64, resourceName corev1.ResourceName, quantity *resource.Quantity) {
	switch resourceName {
	case corev1.ResourceCPU:
		// use millis
		quantity.SetScaled(quantity.MilliValue()*percentage/100, resource.Milli)
	case corev1.ResourceMemory:
		// use mega
		quantity.SetScaled(quantity.ScaledValue(resource.Mega)*percentage/100, resource.Mega)
	default:
		quantity.Set(quantity.Value() * percentage / 100)
	}
}

// IsAllZero returns true if all the values in the ResourceList are zeroes. It returns false otherwise.
func IsAllZero(resources *corev1.ResourceList) bool {
	for _, value := range *resources {
		if !value.IsZero() {
			return false
		}
	}
	return true
}
