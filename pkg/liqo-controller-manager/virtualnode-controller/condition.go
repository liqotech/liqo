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

// ForgeConditionWithMessage sets the condition of the virtual node with a message.
func ForgeConditionWithMessage(
	virtualNode *virtualkubeletv1alpha1.VirtualNode,
	condition virtualkubeletv1alpha1.VirtualNodeConditionType,
	status virtualkubeletv1alpha1.VirtualNodeConditionStatusType,
	msg string) (update bool) {
	for i := range virtualNode.Status.Conditions {
		if virtualNode.Status.Conditions[i].Type != condition {
			continue
		}
		if virtualNode.Status.Conditions[i].Status == status {
			return false
		}
		if (virtualNode.Status.Conditions[i].Status == virtualkubeletv1alpha1.RunningConditionStatusType) &&
			(status == virtualkubeletv1alpha1.CreatingConditionStatusType) {
			return false
		}
		if (virtualNode.Status.Conditions[i].Status == virtualkubeletv1alpha1.DeletingConditionStatusType) &&
			status == virtualkubeletv1alpha1.DrainingConditionStatusType {
			return false
		}
		virtualNode.Status.Conditions[i].Status = status
		virtualNode.Status.Conditions[i].LastTransitionTime = metav1.Now()
		virtualNode.Status.Conditions[i].Message = msg
		return true
	}
	virtualNode.Status.Conditions = append(virtualNode.Status.Conditions,
		virtualkubeletv1alpha1.VirtualNodeCondition{
			Type:               condition,
			Status:             status,
			LastTransitionTime: metav1.Now(),
			Message:            msg,
		})
	return true
}

// ForgeCondition sets the condition of the virtual node.
func ForgeCondition(virtualNode *virtualkubeletv1alpha1.VirtualNode,
	condition virtualkubeletv1alpha1.VirtualNodeConditionType,
	status virtualkubeletv1alpha1.VirtualNodeConditionStatusType) (update bool) {
	return ForgeConditionWithMessage(virtualNode, condition, status, "")
}

// UpdateCondition updates the condition of the virtual node.
func UpdateCondition(ctx context.Context, cl client.Client,
	virtualNode *virtualkubeletv1alpha1.VirtualNode,
	condition virtualkubeletv1alpha1.VirtualNodeConditionType,
	status virtualkubeletv1alpha1.VirtualNodeConditionStatusType) error {
	if ForgeCondition(virtualNode, condition, status) {
		if err := cl.Status().Update(ctx, virtualNode); err != nil {
			return err
		}
	}
	return nil
}

// UpdateConditionWithMessage updates the condition of the virtual node with a message.
func UpdateConditionWithMessage(ctx context.Context, cl client.Client,
	virtualNode *virtualkubeletv1alpha1.VirtualNode,
	condition virtualkubeletv1alpha1.VirtualNodeConditionType,
	status virtualkubeletv1alpha1.VirtualNodeConditionStatusType,
	msg string) error {
	if ForgeConditionWithMessage(virtualNode, condition, status, msg) {
		if err := cl.Status().Update(ctx, virtualNode); err != nil {
			return err
		}
	}
	return nil
}
