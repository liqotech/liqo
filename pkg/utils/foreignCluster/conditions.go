// Copyright 2019-2024 The Liqo Authors
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

package foreigncluster

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// EnsureModuleCondition ensures the status for the given condition.
func EnsureModuleCondition(module *discoveryv1alpha1.Module,
	conditionType discoveryv1alpha1.ConditionType,
	status discoveryv1alpha1.ConditionStatusType,
	reason, message string) {
	for i := range module.Conditions {
		cond := &module.Conditions[i]
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
	module.Conditions = append(module.Conditions,
		discoveryv1alpha1.Condition{
			Type:               conditionType,
			Status:             status,
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            message,
		})
}

// GetStatus returns the status for the given condition. If the condition is not set, it returns the None status.
func GetStatus(conditions []discoveryv1alpha1.Condition, conditionType discoveryv1alpha1.ConditionType) discoveryv1alpha1.ConditionStatusType {
	cond := findCondition(conditions, conditionType)
	if cond != nil {
		return cond.Status
	}
	return discoveryv1alpha1.ConditionStatusNone
}

// GetReason returns the reason for the given condition. If the condition is not set, it returns an empty string.
func GetReason(conditions []discoveryv1alpha1.Condition, conditionType discoveryv1alpha1.ConditionType) string {
	cond := findCondition(conditions, conditionType)
	if cond != nil {
		return cond.Reason
	}
	return ""
}

// GetMessage returns the message for the given condition. If the condition is not set, it returns an empty string.
func GetMessage(conditions []discoveryv1alpha1.Condition, conditionType discoveryv1alpha1.ConditionType) string {
	cond := findCondition(conditions, conditionType)
	if cond != nil {
		return cond.Message
	}
	return ""
}

// findCondition returns a condition given its type.
func findCondition(conditions []discoveryv1alpha1.Condition, conditionType discoveryv1alpha1.ConditionType) *discoveryv1alpha1.Condition {
	for i := range conditions {
		cond := &conditions[i]
		if cond.Type == conditionType {
			return cond
		}
	}
	return nil
}
