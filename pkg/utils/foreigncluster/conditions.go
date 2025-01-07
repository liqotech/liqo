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

package foreigncluster

import (
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

// EnsureGenericCondition ensures the presence of a generic condition in the foreign cluster status.
func EnsureGenericCondition(foreignCluster *liqov1beta1.ForeignCluster,
	conditionType liqov1beta1.ConditionType,
	status liqov1beta1.ConditionStatusType,
	reason, message string) {
	for i := range foreignCluster.Status.Conditions {
		cond := &foreignCluster.Status.Conditions[i]
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
	foreignCluster.Status.Conditions = append(foreignCluster.Status.Conditions,
		liqov1beta1.Condition{
			Type:               conditionType,
			Status:             status,
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            message,
		})
}

// EnsureModuleCondition ensures the presence of a condition in the module.
func EnsureModuleCondition(module *liqov1beta1.Module,
	conditionType liqov1beta1.ConditionType,
	status liqov1beta1.ConditionStatusType,
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
		liqov1beta1.Condition{
			Type:               conditionType,
			Status:             status,
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            message,
		})
}

// DeleteGenericCondition ensure the absence of a generic condition in the foreign cluster status.
func DeleteGenericCondition(foreignCluster *liqov1beta1.ForeignCluster, conditionType liqov1beta1.ConditionType) {
	foreignCluster.Status.Conditions = deleteCondition(foreignCluster.Status.Conditions, conditionType)
}

// DeleteModuleCondition ensure the absence of a condition of the given type in the module.
func DeleteModuleCondition(module *liqov1beta1.Module, conditionType liqov1beta1.ConditionType) {
	module.Conditions = deleteCondition(module.Conditions, conditionType)
}

// GetStatus returns the status for the given condition. If the condition is not set, it returns the None status.
func GetStatus(conditions []liqov1beta1.Condition, conditionType liqov1beta1.ConditionType) liqov1beta1.ConditionStatusType {
	cond := findCondition(conditions, conditionType)
	if cond != nil {
		return cond.Status
	}
	return liqov1beta1.ConditionStatusNone
}

// GetReason returns the reason for the given condition. If the condition is not set, it returns an empty string.
func GetReason(conditions []liqov1beta1.Condition, conditionType liqov1beta1.ConditionType) string {
	cond := findCondition(conditions, conditionType)
	if cond != nil {
		return cond.Reason
	}
	return ""
}

// GetMessage returns the message for the given condition. If the condition is not set, it returns an empty string.
func GetMessage(conditions []liqov1beta1.Condition, conditionType liqov1beta1.ConditionType) string {
	cond := findCondition(conditions, conditionType)
	if cond != nil {
		return cond.Message
	}
	return ""
}

// findCondition returns a condition given its type.
func findCondition(conditions []liqov1beta1.Condition, conditionType liqov1beta1.ConditionType) *liqov1beta1.Condition {
	for i := range conditions {
		cond := &conditions[i]
		if cond.Type == conditionType {
			return cond
		}
	}
	return nil
}

// deleteCondition deletes the condition with the given type from the list of conditions and returns the updated slice.
func deleteCondition(conditions []liqov1beta1.Condition, conditionType liqov1beta1.ConditionType) []liqov1beta1.Condition {
	return slices.DeleteFunc(conditions, func(cond liqov1beta1.Condition) bool {
		return cond.Type == conditionType
	})
}
