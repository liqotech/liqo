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

package liqonodeprovider

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	resourcesMessageSufficient   = "The remote cluster is advertising sufficient resources"
	resourcesMessageInsufficient = "The remote cluster is advertising no/insufficient resources"
)

// UnknownNodeConditions returns an array of node conditions with all unknown status.
func UnknownNodeConditions(cfg *InitConfig) []corev1.NodeCondition {
	conditions := []corev1.NodeCondition{
		*unknownCondition(corev1.NodeReady),
		*unknownCondition(corev1.NodeMemoryPressure),
		*unknownCondition(corev1.NodeDiskPressure),
		*unknownCondition(corev1.NodePIDPressure),
	}
	if cfg.CheckNetworkStatus {
		conditions = append(conditions, *unknownCondition(corev1.NodeNetworkUnavailable))
	}
	return conditions
}

// UpdateNodeCondition updates the specified node condition, depending on the outcome of the specified function.
func UpdateNodeCondition(node *corev1.Node, conditionType corev1.NodeConditionType, conditionStatus func() (corev1.ConditionStatus, string, string)) {
	condition, found := lookupConditionOrCreateUnknown(node, conditionType)

	now := metav1.Now()
	condition.LastHeartbeatTime = now

	// Update the condition
	if status, reason, message := conditionStatus(); status != condition.Status {
		condition.Status = status
		condition.Reason = reason
		condition.Message = message
		condition.LastTransitionTime = now
	}

	// Append the condition if it was not already present in the list.
	if !found {
		node.Status.Conditions = append(node.Status.Conditions, *condition)
	}
}

// nodeReadyStatus returns a function containing the condition information about node readiness.
func nodeReadyStatus(ready bool) func() (corev1.ConditionStatus, string, string) {
	return func() (status corev1.ConditionStatus, reason, message string) {
		if ready {
			return corev1.ConditionTrue, "KubeletReady", "The Liqo Virtual Kubelet is posting ready status"
		}
		return corev1.ConditionFalse, "KubeletNotReady", "The Liqo Virtual Kubelet is currently not ready"
	}
}

// nodeMemoryPressureStatus returns a function containing the condition information about the memory pressure status.
func nodeMemoryPressureStatus(pressure bool) func() (corev1.ConditionStatus, string, string) {
	return func() (status corev1.ConditionStatus, reason, message string) {
		if pressure {
			return corev1.ConditionTrue, "RemoteClusterHasMemoryPressure", resourcesMessageInsufficient
		}
		return corev1.ConditionFalse, "RemoteClusterHasSufficientMemory", resourcesMessageSufficient
	}
}

// nodeDiskPressureStatus returns a function containing the condition information about the disk pressure status.
func nodeDiskPressureStatus(pressure bool) func() (corev1.ConditionStatus, string, string) {
	return func() (status corev1.ConditionStatus, reason, message string) {
		if pressure {
			return corev1.ConditionTrue, "RemoteClusterHasDiskPressure", resourcesMessageInsufficient
		}
		return corev1.ConditionFalse, "RemoteClusterHasNoDiskPressure", resourcesMessageSufficient
	}
}

// nodePIDPressureStatus returns a function containing the condition information about the PID pressure status.
func nodePIDPressureStatus(pressure bool) func() (corev1.ConditionStatus, string, string) {
	return func() (status corev1.ConditionStatus, reason, message string) {
		if pressure {
			return corev1.ConditionTrue, "RemoteClusterHasPIDPressure", resourcesMessageInsufficient
		}
		return corev1.ConditionFalse, "RemoteClusterHasNoPIDPressure", resourcesMessageSufficient
	}
}

// nodeNetworkUnavailableStatus returns a function containing the condition information about the networking status.
func nodeNetworkUnavailableStatus(unavailable bool) func() (corev1.ConditionStatus, string, string) {
	return func() (status corev1.ConditionStatus, reason, message string) {
		if unavailable {
			return corev1.ConditionTrue, "LiqoNetworkingDown", "The Liqo cluster interconnection is down"
		}
		return corev1.ConditionFalse, "LiqoNetworkingUp", "The Liqo cluster interconnection is established"
	}
}

// unknownCondition returns a new condition with unknown status.
func unknownCondition(desired corev1.NodeConditionType) *corev1.NodeCondition {
	return &corev1.NodeCondition{
		Type:   desired,
		Status: corev1.ConditionUnknown,
	}
}

// lookupCondition retrieves a desired condition from a node object, or nil if not found.
func lookupCondition(node *corev1.Node, desired corev1.NodeConditionType) *corev1.NodeCondition {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == desired {
			return &node.Status.Conditions[i]
		}
	}

	return nil
}

// lookupConditionOrCreateUnknown retrieves a desired condition from a node object, or create a new one (with unknown status)
// if not found. An additional boolean field specifies whether the condition was found or not.
func lookupConditionOrCreateUnknown(node *corev1.Node, desired corev1.NodeConditionType) (*corev1.NodeCondition, bool) {
	if condition := lookupCondition(node, desired); condition != nil {
		return condition, true
	}

	// The condition is not immediately added, as it would get copied and the subsequent changes would get lost.
	// Instead, a boolean value is returned to specify it needs to be appended once all modifications are performed.
	return unknownCondition(desired), false
}

// deleteCondition removes a condition from a list of conditions.
func deleteCondition(node *corev1.Node, conditionType corev1.NodeConditionType) {
	node.Status.Conditions = slices.DeleteFunc(node.Status.Conditions, func(cond corev1.NodeCondition) bool {
		return cond.Type == conditionType
	})
}
