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

package utils

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
)

// AddRemoteNamespaceCondition sets newCondition in the conditions slice.
// conditions must be non-nil.
// 1. if the condition of the specified type already exists (all fields of the existing condition are updated to
//    newCondition, LastTransitionTime is set to now if the new status differs from the old status).
// 2. if a condition of the specified type does not exist (LastTransitionTime is set to now() if unset, and newCondition is appended).
func AddRemoteNamespaceCondition(conditions *[]offv1alpha1.RemoteNamespaceCondition,
	newCondition *offv1alpha1.RemoteNamespaceCondition) {
	if conditions == nil {
		return
	}
	existingCondition := FindRemoteNamespaceCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		if newCondition.LastTransitionTime.IsZero() {
			newCondition.LastTransitionTime = metav1.NewTime(time.Now())
		}
		*conditions = append(*conditions, *newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		if !newCondition.LastTransitionTime.IsZero() {
			existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		} else {
			existingCondition.LastTransitionTime = metav1.NewTime(time.Now())
		}
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
}

// RemoveRemoteNamespaceCondition removes the corresponding conditionType from conditions.
// conditions must be non-nil.
func RemoveRemoteNamespaceCondition(conditions *[]offv1alpha1.RemoteNamespaceCondition,
	conditionType offv1alpha1.RemoteNamespaceConditionType) {
	if conditions == nil || len(*conditions) == 0 {
		return
	}
	newConditions := make([]offv1alpha1.RemoteNamespaceCondition, 0, len(*conditions)-1)
	for _, condition := range *conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}
	*conditions = newConditions
}

// FindRemoteNamespaceCondition finds the conditionType in conditions.
func FindRemoteNamespaceCondition(conditions []offv1alpha1.RemoteNamespaceCondition,
	conditionType offv1alpha1.RemoteNamespaceConditionType) *offv1alpha1.RemoteNamespaceCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

// IsStatusConditionTrue returns true when the conditionType is present and set to `corev1.ConditionTrue`.
func IsStatusConditionTrue(conditions []offv1alpha1.RemoteNamespaceCondition,
	conditionType offv1alpha1.RemoteNamespaceConditionType) bool {
	return IsStatusConditionPresentAndEqual(conditions, conditionType, corev1.ConditionTrue)
}

// IsStatusConditionFalse returns true when the conditionType is present and set to `corev1.ConditionFalse`.
func IsStatusConditionFalse(conditions []offv1alpha1.RemoteNamespaceCondition,
	conditionType offv1alpha1.RemoteNamespaceConditionType) bool {
	return IsStatusConditionPresentAndEqual(conditions, conditionType, corev1.ConditionFalse)
}

// IsStatusConditionPresentAndEqual returns true when conditionType is present and equal to status.
func IsStatusConditionPresentAndEqual(conditions []offv1alpha1.RemoteNamespaceCondition,
	conditionType offv1alpha1.RemoteNamespaceConditionType, status corev1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}
