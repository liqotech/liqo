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

package authentication

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
)

// GetCondition returns the condition with the given type.
func GetCondition(resourceSlice *authv1beta1.ResourceSlice,
	conditionType authv1beta1.ResourceSliceConditionType) *authv1beta1.ResourceSliceCondition {
	for i := range resourceSlice.Status.Conditions {
		if resourceSlice.Status.Conditions[i].Type == conditionType {
			return &resourceSlice.Status.Conditions[i]
		}
	}
	return nil
}

// EnsureCondition ensures the condition with the given type, status, reason, and message.
func EnsureCondition(resourceSlice *authv1beta1.ResourceSlice,
	conditionType authv1beta1.ResourceSliceConditionType, status authv1beta1.ResourceSliceConditionStatus,
	reason, message string) controllerutil.OperationResult {
	condition := GetCondition(resourceSlice, conditionType)
	if condition != nil {
		if condition.Status != status || reason != condition.Reason || message != condition.Message {
			condition.Status = status
			condition.Reason = reason
			condition.Message = message
			condition.LastTransitionTime = metav1.Now()
			return controllerutil.OperationResultUpdated
		}
		return controllerutil.OperationResultNone
	}

	resourceSlice.Status.Conditions = append(resourceSlice.Status.Conditions, authv1beta1.ResourceSliceCondition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
	return controllerutil.OperationResultCreated
}
