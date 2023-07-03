// Copyright 2019-2023 The Liqo Authors
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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

// VkConditionMap is a map of virtual node conditions.
type VkConditionMap map[virtualkubeletv1alpha1.VirtualNodeConditionType]VkCondition

// VkCondition is a virtual node condition.
type VkCondition struct {
	Status  virtualkubeletv1alpha1.VirtualNodeConditionStatusType
	Message string
}

// ForgeCondition forges a virtual node condition.
func ForgeCondition(
	virtualNode *virtualkubeletv1alpha1.VirtualNode,
	vkConditions VkConditionMap) (update bool) {
	for nameCondition, vkCondition := range vkConditions {
		for i := range virtualNode.Status.Conditions {
			if virtualNode.Status.Conditions[i].Type != nameCondition {
				continue
			}
			if virtualNode.Status.Conditions[i].Status == vkCondition.Status {
				return false
			}
			if (virtualNode.Status.Conditions[i].Status == virtualkubeletv1alpha1.RunningConditionStatusType) &&
				(vkCondition.Status == virtualkubeletv1alpha1.CreatingConditionStatusType) {
				return false
			}
			if (virtualNode.Status.Conditions[i].Status == virtualkubeletv1alpha1.DeletingConditionStatusType) &&
				vkCondition.Status == virtualkubeletv1alpha1.DrainingConditionStatusType {
				return false
			}
			virtualNode.Status.Conditions[i].Status = vkCondition.Status
			virtualNode.Status.Conditions[i].LastTransitionTime = metav1.Now()
			virtualNode.Status.Conditions[i].Message = vkCondition.Message
			return true
		}
		virtualNode.Status.Conditions = append(virtualNode.Status.Conditions,
			virtualkubeletv1alpha1.VirtualNodeCondition{
				Type:               nameCondition,
				Status:             vkCondition.Status,
				LastTransitionTime: metav1.Now(),
				Message:            vkCondition.Message,
			})
	}
	return true
}

// UpdateCondition updates the condition of the virtual node.
func UpdateCondition(ctx context.Context, cl client.Client,
	virtualNode *virtualkubeletv1alpha1.VirtualNode,
	vkConditions VkConditionMap,
) error {
	if ForgeCondition(virtualNode, vkConditions) {
		if err := cl.Status().Update(ctx, virtualNode); err != nil {
			return err
		}
	}
	return nil
}

// GetCondition returns the condition of the virtual node.
func GetCondition(virtualNode *virtualkubeletv1alpha1.VirtualNode,
	condition virtualkubeletv1alpha1.VirtualNodeConditionType) *virtualkubeletv1alpha1.VirtualNodeCondition {
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
