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

package pod

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

// IsPodReady returns true if a pod is ready; false otherwise. It also returns a reason (as provided by Kubernetes).
func IsPodReady(pod *corev1.Pod) (ready bool, reason string) {
	conditions := pod.Status.Conditions
	for i := range conditions {
		condition := conditions[i]
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue, condition.Reason
		}
	}
	return false, "no conditions in pod status"
}

// GetPodCondition extracts the provided condition from the given status and returns that.
func GetPodCondition(status *corev1.PodStatus, conditionType corev1.PodConditionType) *corev1.PodCondition {
	if status == nil {
		return nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return &status.Conditions[i]
		}
	}
	return nil
}

// IsPodSpecEqual returns whether two pod specs are equal according to the fields that
// can be modified after start-up time. Refer to the following link for more information:
// https://kubernetes.io/docs/concepts/workloads/pods/#pod-update-and-replacement
// This function is implemented custom instead of relying on reflect.DeepEqual or alike for
// performance reasons, given the possibly high execution rate when dealing with pod reflection.
func IsPodSpecEqual(previous, updated *corev1.PodSpec) bool {
	// The only fields that can be mutated are:
	// * spec.containers[*].image
	// * spec.initContainers[*].image
	// * spec.activeDeadlineSeconds
	// * spec.tolerations (only new entries can be added)
	return AreContainersEqual(previous.Containers, updated.Containers) &&
		AreContainersEqual(previous.InitContainers, updated.InitContainers) &&
		ptr.Equal(previous.ActiveDeadlineSeconds, updated.ActiveDeadlineSeconds) &&
		len(previous.Tolerations) == len(updated.Tolerations)
}

// CheckShadowPodUpdate returns whether updated equals previous, except for the fields that are allowed to be updated.
// The updated object gets mutated, and a deepcopy shall be performed if it needs to be reused.
func CheckShadowPodUpdate(previous, updated *corev1.PodSpec) bool {
	// The only fields that can be mutated are:
	// * spec.containers[*].image
	// * spec.initContainers[*].image
	// * spec.activeDeadlineSeconds
	// * spec.tolerations (only new entries can be added)
	for i := range updated.Containers {
		updated.Containers[i].Image = previous.Containers[i].Image
	}
	for i := range updated.InitContainers {
		updated.InitContainers[i].Image = previous.InitContainers[i].Image
	}
	updated.ActiveDeadlineSeconds = previous.ActiveDeadlineSeconds
	updated.Tolerations = previous.Tolerations
	return reflect.DeepEqual(previous, updated)
}

// AreContainersEqual returns whether two container lists are equal according to the
// fields that can be modified after start-up time (i.e. the image field).
func AreContainersEqual(previous, updated []corev1.Container) bool {
	if len(previous) != len(updated) {
		return false
	}

outer:
	for i := range previous {
		for j := range updated {
			if previous[i].Name == updated[j].Name {
				if previous[i].Image == updated[j].Image {
					continue outer
				}
				return false
			}
		}
		return false
	}

	return true
}

// ForgeContainerResources forges the container resource requirements, leaving unset the ones not specified.
func ForgeContainerResources(cpuRequests, cpuLimits, ramRequests, ramLimits resource.Quantity) corev1.ResourceRequirements {
	configure := func(rl corev1.ResourceList, key corev1.ResourceName, value resource.Quantity) {
		if !value.IsZero() {
			rl[key] = value
		}
	}

	requirements := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	configure(requirements.Requests, corev1.ResourceCPU, cpuRequests)
	configure(requirements.Requests, corev1.ResourceMemory, ramRequests)
	configure(requirements.Limits, corev1.ResourceCPU, cpuLimits)
	configure(requirements.Limits, corev1.ResourceMemory, ramLimits)

	return requirements
}

// ServiceAccountName returns the name of the service account, or default if not set.
// Indeed, the ServiceAccountName field in the pod specifications is optional, and empty means default.
func ServiceAccountName(pod *corev1.Pod) string {
	if pod.Spec.ServiceAccountName != "" {
		return pod.Spec.ServiceAccountName
	}

	return "default"
}
