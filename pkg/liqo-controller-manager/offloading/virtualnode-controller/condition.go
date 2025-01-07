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

package virtualnodectrl

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

// VnConditionMap is a map of virtual node conditions.
type VnConditionMap map[offloadingv1beta1.VirtualNodeConditionType]VnCondition

// VnCondition is a virtual node condition.
type VnCondition struct {
	Status  offloadingv1beta1.VirtualNodeConditionStatusType
	Message string
}

// ForgeCondition forges a virtual node condition.
func ForgeCondition(
	virtualNode *offloadingv1beta1.VirtualNode,
	vnConditions VnConditionMap) {
	for nameCondition, vnCondition := range vnConditions {
		for i := range virtualNode.Status.Conditions {
			if virtualNode.Status.Conditions[i].Type != nameCondition {
				continue
			}
			if virtualNode.Status.Conditions[i].Status == vnCondition.Status {
				return
			}
			if (virtualNode.Status.Conditions[i].Status == offloadingv1beta1.RunningConditionStatusType) &&
				(vnCondition.Status == offloadingv1beta1.CreatingConditionStatusType) {
				return
			}
			if (virtualNode.Status.Conditions[i].Status == offloadingv1beta1.NoneConditionStatusType) &&
				(vnCondition.Status == offloadingv1beta1.DrainingConditionStatusType) {
				return
			}
			if (virtualNode.Status.Conditions[i].Status == offloadingv1beta1.NoneConditionStatusType) &&
				(vnCondition.Status == offloadingv1beta1.DeletingConditionStatusType) {
				return
			}
			if (virtualNode.Status.Conditions[i].Status == offloadingv1beta1.DeletingConditionStatusType) &&
				vnCondition.Status == offloadingv1beta1.DrainingConditionStatusType {
				return
			}
			virtualNode.Status.Conditions[i].Status = vnCondition.Status
			virtualNode.Status.Conditions[i].LastTransitionTime = metav1.Now()
			virtualNode.Status.Conditions[i].Message = vnCondition.Message
		}
		virtualNode.Status.Conditions = append(virtualNode.Status.Conditions,
			offloadingv1beta1.VirtualNodeCondition{
				Type:               nameCondition,
				Status:             vnCondition.Status,
				LastTransitionTime: metav1.Now(),
				Message:            vnCondition.Message,
			})
	}
}

// GetCondition returns the condition of the virtual node.
func GetCondition(virtualNode *offloadingv1beta1.VirtualNode,
	condition offloadingv1beta1.VirtualNodeConditionType) *offloadingv1beta1.VirtualNodeCondition {
	if virtualNode == nil {
		return nil
	}
	for i := range virtualNode.Status.Conditions {
		if virtualNode.Status.Conditions[i].Type == condition {
			return &virtualNode.Status.Conditions[i]
		}
	}
	return nil
}
