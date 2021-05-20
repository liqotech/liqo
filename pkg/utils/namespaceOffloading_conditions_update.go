package utils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
)

// UpdateNamespaceOffloadingCondition updates the content of the given condition for the selected Cluster ID.
func UpdateNamespaceOffloadingCondition(status *offv1alpha1.NamespaceOffloadingStatus,
	newCondition *offv1alpha1.RemoteNamespaceCondition, clusterID string) bool {
	if status.RemoteNamespacesConditions == nil {
		status.RemoteNamespacesConditions = map[string]offv1alpha1.RemoteNamespaceConditions{}
	}

	oldCondition := GetNamespaceOffloadingCondition(status.RemoteNamespacesConditions[clusterID], newCondition.Type)

	if oldCondition == nil {
		newCondition.LastTransitionTime = metav1.Now()
		status.RemoteNamespacesConditions[clusterID] = append(status.RemoteNamespacesConditions[clusterID],
			*newCondition)
		return true
	}

	if oldCondition.Status != newCondition.Status ||
		oldCondition.Message != newCondition.Message || oldCondition.Reason != newCondition.Reason {
		if oldCondition.Status != newCondition.Status {
			oldCondition.LastTransitionTime = metav1.Now()
		}
		oldCondition.Status = newCondition.Status
		oldCondition.Reason = newCondition.Reason
		oldCondition.Message = newCondition.Message
		return true
	}

	return false
}

// GetNamespaceOffloadingCondition get the content of the specific condition.
func GetNamespaceOffloadingCondition(conditions []offv1alpha1.RemoteNamespaceCondition,
	conditionType offv1alpha1.RemoteNamespaceConditionType) *offv1alpha1.RemoteNamespaceCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &(conditions[i])
		}
	}
	return nil
}
