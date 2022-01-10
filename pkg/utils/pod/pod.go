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

package pod

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
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
		pointer.Int64Equal(previous.ActiveDeadlineSeconds, updated.ActiveDeadlineSeconds) &&
		len(previous.Tolerations) == len(updated.Tolerations)
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
