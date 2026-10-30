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

package pod

import (
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
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

// MarkNotReady sets the Ready condition of the given pod status to False, adding it if missing.
func MarkNotReady(status *corev1.PodStatus) {
	cond := GetPodCondition(status, corev1.PodReady)
	if cond == nil {
		status.Conditions = append(status.Conditions, corev1.PodCondition{
			Type:               corev1.PodReady,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: metav1.NewTime(time.Now()),
		})
		return
	}
	cond.Status = corev1.ConditionFalse
}

// MarkDisrupted sets the DisruptionTarget condition of the given pod status to True with the given reason, adding it if
// missing: the pod is terminating or failed because of a disruption, not because of its own containers.
func MarkDisrupted(status *corev1.PodStatus, reason string) {
	cond := GetPodCondition(status, corev1.DisruptionTarget)
	if cond == nil {
		status.Conditions = append(status.Conditions, corev1.PodCondition{
			Type:               corev1.DisruptionTarget,
			Status:             corev1.ConditionTrue,
			Reason:             reason,
			LastTransitionTime: metav1.NewTime(time.Now()),
		})
		return
	}
	if cond.Status != corev1.ConditionTrue {
		cond.LastTransitionTime = metav1.NewTime(time.Now())
	}
	cond.Status = corev1.ConditionTrue
	cond.Reason = reason
}

// IsDeletedByOthers returns whether the given offloaded pod is being deleted while its shadowpod is not, that is the
// deletion has not been requested through the shadowpod (e.g., the pod has been deleted or evicted in the remote cluster).
// Such a pod transiently reaches a terminal phase while terminating, which must not be taken as its completion.
func IsDeletedByOthers(pod *corev1.Pod, shadow *offloadingv1beta1.ShadowPod) bool {
	return pod != nil && !pod.DeletionTimestamp.IsZero() && shadow != nil && shadow.DeletionTimestamp.IsZero()
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
