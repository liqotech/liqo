// Copyright 2019-2025 The Liqo Authors
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

package util

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	podutils "github.com/liqotech/liqo/pkg/utils/pod"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// PodType -> defines the type of a pod (local/remote).
type PodType string

const (
	// PodLocal -> the pod is local.
	PodLocal = "local"
	// PodRemote -> the pod is remote.
	PodRemote = "remote"
)

// IsPodUp waits for a specific namespace/podName to be ready. It returns true if the pod within the timeout, false otherwise.
func IsPodUp(ctx context.Context, clientset kubernetes.Interface, namespace, podName string, podType PodType) bool {
	klog.Infof("checking if %s pod %s/%s is ready", podType, namespace, podName)
	podToCheck, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("an error occurred while getting %s pod %s/%s: %v", podType, namespace, podName, err)
		return false
	}

	ready, reason := podutils.IsPodReady(podToCheck)
	message := "ready"
	if !ready {
		message = "NOT ready"
	}

	klog.Infof("%s pod %s/%s is %s (reason: %s)", podType, podToCheck.Namespace, podToCheck.Name, message, reason)
	return ready
}

// ArePodsUp check if all the pods of a specific namespace are ready. It returns a list of ready pods, a list of unready
// pods and occurred errors.
func ArePodsUp(ctx context.Context, clientset kubernetes.Interface, namespace string) (ready, notReady []string, retErr error) {
	pods, retErr := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if retErr != nil {
		klog.Error(retErr)
		return nil, nil, retErr
	}
	for index := range pods.Items {
		if ready, _ := podutils.IsPodReady(&pods.Items[index]); !ready {
			notReady = append(notReady, pods.Items[index].Name)
		}
		ready = append(ready, pods.Items[index].Name)
	}
	return ready, notReady, nil
}

// NumPodsInTenantNs returns the number of pods that should be present in a tenant namespace.
func NumPodsInTenantNs(networkingEnabled bool, role liqov1beta1.RoleType, gwReplicas, vkReplicas int) int {
	count := 0
	// If the network is enabled, it should have the gateway pod.
	if networkingEnabled {
		count += gwReplicas
	}
	// If the cluster is a consumer, it should have the virtual-kubelet pod.
	if fcutils.IsConsumer(role) {
		count += vkReplicas
	}
	return count
}

// NumTenantNamespaces returns the number of tenant namespaces that should be present in a cluster.
func NumTenantNamespaces(numPeeredConsumers, numPeeredProviders int, role liqov1beta1.RoleType) int {
	switch {
	case fcutils.IsConsumer(role):
		return numPeeredProviders
	case fcutils.IsProvider(role):
		return numPeeredConsumers
	default:
		return 0
	}
}

// ResourceRequirements returns the default resource requirements for a pod during tests.
func ResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{Limits: corev1.ResourceList{
		corev1.ResourceCPU:    *k8sresource.NewScaledQuantity(250, k8sresource.Milli),
		corev1.ResourceMemory: *k8sresource.NewScaledQuantity(100, k8sresource.Mega),
	}}
}

// PodOption is a function that modifies a Pod.
type PodOption func(*corev1.Pod)

// RemotePodOption sets the Pod to be scheduled on remote nodes.
func RemotePodOption(virtualNode bool, nodeName *string) PodOption {
	var nodeSelectors []corev1.NodeSelectorRequirement
	if virtualNode {
		nodeSelectors = append(nodeSelectors, corev1.NodeSelectorRequirement{
			Key:      consts.TypeLabel,
			Operator: corev1.NodeSelectorOpExists,
		})
	}

	node := ptr.Deref(nodeName, "")

	return func(pod *corev1.Pod) {
		pod.Spec.NodeName = node

		if len(nodeSelectors) > 0 {
			if pod.Spec.Affinity == nil {
				pod.Spec.Affinity = &corev1.Affinity{}
			}
			if pod.Spec.Affinity.NodeAffinity == nil {
				pod.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
			}
			if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			}
			pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{
				{
					MatchExpressions: nodeSelectors,
				},
			}
		}
	}
}

// EnforcePod creates or updates a Pod with the given name in the given namespace.
func EnforcePod(ctx context.Context, cl client.Client, namespace, name string, options ...PodOption) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return Second(resource.CreateOrUpdate(ctx, cl, pod, func() error {
		pod.Spec = corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            name,
					Image:           "nginx",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
		}

		for _, opt := range options {
			opt(pod)
		}

		return nil
	}))
}
