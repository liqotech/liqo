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

package resourcerequestoperator

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	resourcemonitors "github.com/liqotech/liqo/pkg/liqo-controller-manager/resource-request-controller/resource-monitors"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// DefaultScaleFactor defines the amount of scaled resources to be computed in resourceOffers.
const DefaultScaleFactor = .5

// createNewStorageClass creates a new storage class with name *storageClassName* and creates it using the *clientset* client.
func createNewStorageClass(ctx context.Context, clientset kubernetes.Interface,
	storageClassName, provisioner string, defaultAnnotation bool) (*storagev1.StorageClass, error) {
	storageClass := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageClassName,
		},
		Provisioner: provisioner,
	}

	if defaultAnnotation {
		storageClass.Annotations = map[string]string{
			"storageclass.kubernetes.io/is-default-class": "true",
		}
	}

	return clientset.StorageV1().StorageClasses().Create(ctx, storageClass, metav1.CreateOptions{})
}

// createNewNode forges a new node with name *nodeName* and creates it using the *clientset* client.
func createNewNode(ctx context.Context, nodeName string, virtual bool, clientset kubernetes.Interface) (*corev1.Node, error) {
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

// createNewPod forges a new pod with name *podName* and creates it using the *clientset* client.
func createNewPod(ctx context.Context, podName, clusterID string, shadow bool, clientset kubernetes.Interface) (*corev1.Pod, error) {
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
		pod.Labels[forge.LiqoOriginClusterIDKey] = clusterID
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
		Phase: corev1.PodRunning,
	}
	pod, err = clientset.CoreV1().Pods("default").UpdateStatus(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return pod, nil
}

// setPodPhase enforces a status ready/not ready to a pod passed as the *pod* parameter. The readiness/not readiness is
// enforced by the *status* bool (true is ready, false is not ready).
func setPodPhase(ctx context.Context, pod *corev1.Pod, phase corev1.PodPhase, clientset kubernetes.Interface) (*corev1.Pod, error) {
	pod.Status.Phase = phase
	return clientset.CoreV1().Pods("default").UpdateStatus(ctx, pod, metav1.UpdateOptions{})
}

// setNodeReadyStatus enforces a status ready/not ready to a node passed as the *node* parameter. The readiness/not readiness is
// enforced by the *status* bool (true is ready, false is not ready).
func setNodeReadyStatus(ctx context.Context, node *corev1.Node, status bool, clientset kubernetes.Interface) (*corev1.Node, error) {
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

// checkResourceOfferUpdate returns false if the (1) resource offer does not exist or the get returns an error (2) if the scaled quantity
// available in the resourceOffer is not equal to the one present in the cluster. It returns true otherwise.
func checkResourceOfferUpdate(ctx context.Context, homeCluster discoveryv1alpha1.ClusterIdentity,
	nodeResources, podResources []corev1.ResourceList, k8sClient client.Client) bool {
	offer := &sharingv1alpha1.ResourceOffer{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      homeCluster.ClusterName,
		Namespace: ResourcesNamespace,
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
		resourcemonitors.ScaleResources(resourceName, &toCheck, DefaultScaleFactor)
		if quantity.Cmp(toCheck) != 0 {
			return false
		}
	}
	return true
}

// isAllZero returns true if all the values in the ResourceList are zeroes. It returns false otherwise.
func isAllZero(resources *corev1.ResourceList) bool {
	for _, value := range *resources {
		if !value.IsZero() {
			return false
		}
	}
	return true
}
