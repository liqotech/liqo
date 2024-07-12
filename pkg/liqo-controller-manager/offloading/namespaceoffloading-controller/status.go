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

package nsoffctrl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
)

// enforceStatus realigns the status of the NamespaceOffloading, depending on that of the NamespaceMaps.
func (r *NamespaceOffloadingReconciler) enforceStatus(ctx context.Context, nsoff *offv1alpha1.NamespaceOffloading,
	nsmaps map[string]*offv1alpha1.NamespaceMap) error {
	nsoff.Status.RemoteNamespaceName = r.remoteNamespaceName(nsoff)

	// Update the observed generation.
	nsoff.Status.ObservedGeneration = nsoff.Generation

	// Remove the conditions for the clusters which do no longer exist.
	ensureRemoteConditionsConsistence(nsoff, nsmaps)

	// Fill the conditions corresponding to each remote cluster.
	required, ready, failed := setRemoteConditionsForEveryCluster(nsoff, nsmaps)

	// Configure the global status given the conditions.
	setNamespaceOffloadingStatus(nsoff, required, ready, failed)

	// Update the status just once at the end of the logic.
	if err := r.Status().Update(ctx, nsoff); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// remoteNamespaceName returns the remapped name corresponding to a given namespace.
func (r *NamespaceOffloadingReconciler) remoteNamespaceName(nsoff *offv1alpha1.NamespaceOffloading) string {
	switch nsoff.Spec.NamespaceMappingStrategy {
	case offv1alpha1.EnforceSameNameMappingStrategyType:
		return nsoff.Namespace
	case offv1alpha1.DefaultNameMappingStrategyType:
		return nsoff.Namespace + "-" + foreignclusterutils.UniqueName(r.LocalCluster)
	case offv1alpha1.SelectedNameMappingStrategyType:
		return nsoff.Spec.RemoteNamespaceName
	default:
		klog.Errorf("NamespaceOffloading %q: unknown NamespaceMappingStrategy %q, falling back to %q",
			klog.KObj(nsoff), nsoff.Spec.NamespaceMappingStrategy, offv1alpha1.DefaultNameMappingStrategyType)
		return nsoff.Namespace + "-" + foreignclusterutils.UniqueName(r.LocalCluster)
	}
}

// ensureRemoteConditionsConsistence checks for every remote condition of the NamespaceOffloading resource that the
// corresponding NamespaceMap is still there. If the peering is deleted also the corresponding remote condition
// must be deleted.
func ensureRemoteConditionsConsistence(nsoff *offv1alpha1.NamespaceOffloading, nsmaps map[string]*offv1alpha1.NamespaceMap) {
outer:
	for nmname := range nsoff.Status.RemoteNamespacesConditions {
		for _, nsmap := range nsmaps {
			if nsmap.GetName() == nmname {
				continue outer
			}
		}

		delete(nsoff.Status.RemoteNamespacesConditions, nmname)
		klog.V(4).Infof("NamespaceOffloading %q: remote cluster %q no longer available", klog.KObj(nsoff), nmname)
	}
}

// setRemoteConditionsForEveryCluster configures the conditions depending on whether the namespace has been offloaded, and its status.
// It additionally returns the number of clusters selected as targets for offloading, and the number of ready and failed ones.
func setRemoteConditionsForEveryCluster(nsoff *offv1alpha1.NamespaceOffloading,
	nsmaps map[string]*offv1alpha1.NamespaceMap) (requestedCount, readyCount, failedCount uint) {
	if nsoff.Status.RemoteNamespacesConditions == nil {
		nsoff.Status.RemoteNamespacesConditions = map[string]offv1alpha1.RemoteNamespaceConditions{}
	}

	for _, nsmap := range nsmaps {
		// Get the information for the NamespaceOffloadingRequired condition.
		_, requested := nsmap.Spec.DesiredMapping[nsoff.Namespace]
		if requested {
			requestedCount++
		}

		// Get the information for the NamespaceReady condition.
		var phase offv1alpha1.MappingPhase
		if mapping, ok := nsmap.Status.CurrentMapping[nsoff.Namespace]; ok {
			phase = mapping.Phase
			if phase == offv1alpha1.MappingAccepted {
				readyCount++
			} else if phase == offv1alpha1.MappingCreationLoopBackOff {
				failedCount++
			}
		}

		if !nsoff.GetDeletionTimestamp().IsZero() && !requested && phase == "" {
			// Remove all conditions in case the NamespaceOffloading is being deleted, and offloading correctly terminated.
			delete(nsoff.Status.RemoteNamespacesConditions, nsmap.GetName())
		} else {
			// Otherwise, set the appropriate conditions.
			setRemoteCondition(nsoff, nsmap.GetName(), nsoffRequiredCondition(requested))
			if requested || phase != "" {
				setRemoteCondition(nsoff, nsmap.GetName(), nsoffReadyCondition(phase))
			}
		}
	}

	return requestedCount, readyCount, failedCount
}

// setRemoteCondition configures the conditions referring to a single remote cluster,
// depending on whether the namespace has been offloaded, and its status.
func setRemoteCondition(nsoff *offv1alpha1.NamespaceOffloading, nmname string, condition *offv1alpha1.RemoteNamespaceCondition) {
	// Iterate over the existing conditions, and check whether it is up-to-date
	conditions := nsoff.Status.RemoteNamespacesConditions[nmname]
	for i := range conditions {
		if conditions[i].Type == condition.Type {
			if conditions[i].Status == condition.Status && conditions[i].Reason == condition.Reason {
				// The condition is already up-to-date, and there is nothing to do
				return
			}

			// Otherwise, overwrite the condition with the new one
			conditions[i] = *condition
			return
		}
	}

	// Append the condition if not already present
	nsoff.Status.RemoteNamespacesConditions[nmname] = append(conditions, *condition)
}

// nsoffRequiredCondition returns a condition stating whether the namespace shall be offladed to the remote cluster or not.
func nsoffRequiredCondition(required bool) *offv1alpha1.RemoteNamespaceCondition {
	condition := &offv1alpha1.RemoteNamespaceCondition{Type: offv1alpha1.NamespaceOffloadingRequired, LastTransitionTime: metav1.Now()}

	if required {
		condition.Status = corev1.ConditionTrue
		condition.Reason = "ClusterSelected"
		condition.Message = "The remote cluster has been selected through the ClusterSelector field"
	} else {
		condition.Status = corev1.ConditionFalse
		condition.Reason = "ClusterNotSelected"
		condition.Message = "The remote cluster has not been selected through the ClusterSelector field"
	}

	return condition
}

// nsoffRequiredCondition returns a condition stating the offloading status, based on the corresponding NamespaceMap.
func nsoffReadyCondition(phase offv1alpha1.MappingPhase) *offv1alpha1.RemoteNamespaceCondition {
	condition := &offv1alpha1.RemoteNamespaceCondition{Type: offv1alpha1.NamespaceReady, LastTransitionTime: metav1.Now()}

	switch {
	case phase == offv1alpha1.MappingAccepted:
		condition.Status = corev1.ConditionTrue
		condition.Reason = "NamespaceCreated"
		condition.Message = "Namespace correctly offloaded to the remote cluster"
	case phase == offv1alpha1.MappingCreationLoopBackOff:
		condition.Status = corev1.ConditionFalse
		condition.Reason = "CreationLoopBackOff"
		condition.Message = "Some problems occurred during remote namespace creation"
	case phase == offv1alpha1.MappingTerminating:
		condition.Status = corev1.ConditionFalse
		condition.Reason = "Terminating"
		condition.Message = "The remote namespace is being deleted"
	default:
		condition.Status = corev1.ConditionFalse
		condition.Reason = "Creating"
		condition.Message = "The remote namespace is being created"
	}

	return condition
}

// setNamespaceOffloadingStatus sets the global offloading status according to the remote namespace conditions.
func setNamespaceOffloadingStatus(nsoff *offv1alpha1.NamespaceOffloading, required, ready, failed uint) {
	switch {
	case !nsoff.DeletionTimestamp.IsZero():
		nsoff.Status.OffloadingPhase = offv1alpha1.TerminatingOffloadingPhaseType
	case required == 0:
		nsoff.Status.OffloadingPhase = offv1alpha1.NoClusterSelectedOffloadingPhaseType
	case ready == required:
		nsoff.Status.OffloadingPhase = offv1alpha1.ReadyOffloadingPhaseType
	case failed == required:
		nsoff.Status.OffloadingPhase = offv1alpha1.AllFailedOffloadingPhaseType
	case failed > 0:
		nsoff.Status.OffloadingPhase = offv1alpha1.SomeFailedOffloadingPhaseType
	default:
		nsoff.Status.OffloadingPhase = offv1alpha1.InProgressOffloadingPhaseType
	}
}
