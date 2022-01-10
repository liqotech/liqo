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

package peeringconditionsutils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// EnsureStatus ensures the status for the given peering condition.
func EnsureStatus(
	foreignCluster *discoveryv1alpha1.ForeignCluster,
	conditionType discoveryv1alpha1.PeeringConditionType,
	status discoveryv1alpha1.PeeringConditionStatusType,
	reason, message string) {
	for i := range foreignCluster.Status.PeeringConditions {
		cond := &foreignCluster.Status.PeeringConditions[i]
		if cond.Type == conditionType {
			if cond.Status != status || reason != cond.Reason || message != cond.Message {
				cond.Status = status
				cond.LastTransitionTime = metav1.Now()
				cond.Reason = reason
				cond.Message = message
			}
			return
		}
	}

	// if the type has not been found in the list, add it
	foreignCluster.Status.PeeringConditions = append(foreignCluster.Status.PeeringConditions,
		discoveryv1alpha1.PeeringCondition{
			Type:               conditionType,
			Status:             status,
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            message,
		})
}

// GetStatus returns the status for the given peering condition. If the condition is not set,
// it returns the None status.
func GetStatus(foreignCluster *discoveryv1alpha1.ForeignCluster,
	conditionType discoveryv1alpha1.PeeringConditionType) discoveryv1alpha1.PeeringConditionStatusType {
	cond := findCondition(foreignCluster, conditionType)
	if cond != nil {
		return cond.Status
	}
	return discoveryv1alpha1.PeeringConditionStatusNone
}

// GetReason returns the reason for the given peering condition. If the condition is not set,
// it returns an empty string.
func GetReason(foreignCluster *discoveryv1alpha1.ForeignCluster,
	conditionType discoveryv1alpha1.PeeringConditionType) string {
	cond := findCondition(foreignCluster, conditionType)
	if cond != nil {
		return cond.Reason
	}
	return ""
}

// GetMessage returns the message for the given peering condition. If the condition is not set,
// it returns an empty string.
func GetMessage(foreignCluster *discoveryv1alpha1.ForeignCluster,
	conditionType discoveryv1alpha1.PeeringConditionType) string {
	cond := findCondition(foreignCluster, conditionType)
	if cond != nil {
		return cond.Message
	}
	return ""
}

// findCondition returns a condition given its type.
func findCondition(foreignCluster *discoveryv1alpha1.ForeignCluster,
	conditionType discoveryv1alpha1.PeeringConditionType) *discoveryv1alpha1.PeeringCondition {
	for i := range foreignCluster.Status.PeeringConditions {
		cond := &foreignCluster.Status.PeeringConditions[i]
		if cond.Type == conditionType {
			return cond
		}
	}
	return nil
}
