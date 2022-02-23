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

package offloadingstatuscontroller

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	mapsv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
)

// mapPhaseToRemoteNamespaceCondition selects the right remote condition according to the remote namespace
// phase obtained by means of NamespaceMap.Status.CurrentMapping.
func mapPhaseToRemoteNamespaceCondition(phase mapsv1alpha1.MappingPhase) offv1alpha1.RemoteNamespaceCondition {
	var remoteCondition offv1alpha1.RemoteNamespaceCondition
	switch {
	case phase == mapsv1alpha1.MappingAccepted:
		remoteCondition = offv1alpha1.RemoteNamespaceCondition{
			Type:    offv1alpha1.NamespaceReady,
			Status:  corev1.ConditionTrue,
			Reason:  "RemoteNamespaceCreated",
			Message: "Namespace correctly offloaded on this cluster",
		}
	case phase == mapsv1alpha1.MappingCreationLoopBackOff:
		remoteCondition = offv1alpha1.RemoteNamespaceCondition{
			Type:    offv1alpha1.NamespaceReady,
			Status:  corev1.ConditionFalse,
			Reason:  "CreationLoopBackOff",
			Message: "Some problems occurred during remote Namespace creation",
		}
	case phase == mapsv1alpha1.MappingTerminating:
		remoteCondition = offv1alpha1.RemoteNamespaceCondition{
			Type:    offv1alpha1.NamespaceReady,
			Status:  corev1.ConditionFalse,
			Reason:  "TerminatingNamespace",
			Message: "The remote Namespace is requested to be deleted",
		}
	// If phase is not specified.
	default:
		remoteCondition = offv1alpha1.RemoteNamespaceCondition{
			Type:    offv1alpha1.NamespaceOffloadingRequired,
			Status:  corev1.ConditionFalse,
			Reason:  "ClusterNotSelected",
			Message: "You have not selected this cluster through ClusterSelector fields",
		}
	}
	return remoteCondition
}

// assignClusterRemoteCondition sets the right remote namespace condition according to the remote namespace phase
// written in NamespaceMap.Status.CurrentMapping.
// If phase==nil the remote namespace condition OffloadingRequired=False is set.
func assignClusterRemoteCondition(noff *offv1alpha1.NamespaceOffloading, phase mapsv1alpha1.MappingPhase, clusterID string) {
	if noff.Status.RemoteNamespacesConditions == nil {
		noff.Status.RemoteNamespacesConditions = map[string]offv1alpha1.RemoteNamespaceConditions{}
	}

	newCondition := mapPhaseToRemoteNamespaceCondition(phase)
	// if the condition is already there, do nothing
	if liqoutils.IsStatusConditionPresentAndEqual(noff.Status.RemoteNamespacesConditions[clusterID], newCondition.Type, newCondition.Status) {
		return
	}
	var remoteConditions []offv1alpha1.RemoteNamespaceCondition
	liqoutils.AddRemoteNamespaceCondition(&remoteConditions, &newCondition)
	noff.Status.RemoteNamespacesConditions[clusterID] = remoteConditions
	klog.Infof("Remote condition of type '%s' with Status '%s' for the remote namespace '%s' associated with the cluster '%s'",
		remoteConditions[0].Type, remoteConditions[0].Status, noff.Namespace, clusterID)
}

// todo: at the moment the global status InProgress is not implemented, at every reconcile the controller sets a global
//       OffloadingStatus that reflects the current Status of NamespaceMaps
// If the NamespaceMap has a remote Status for that remote Namespace, the right remote condition is set according to the
// remote Namespace Phase. If there is no remote Status in the NamespaceMap, the OffloadingRequired=false condition is set
// this condition could be only transient until the NamespaceMap Status is updated or permanent if the local Namespace
// is not requested to be offloaded inside this cluster.
func setRemoteConditionsForEveryCluster(noff *offv1alpha1.NamespaceOffloading, nml *mapsv1alpha1.NamespaceMapList) {
	for i := range nml.Items {
		if remoteNamespaceStatus, ok := nml.Items[i].Status.CurrentMapping[noff.Namespace]; ok {
			assignClusterRemoteCondition(noff, remoteNamespaceStatus.Phase, nml.Items[i].GetName())
			continue
		}
		// Two cases in which there are no entry in NamespaceMap Status:
		// - when the local namespace is not offloaded inside this cluster.
		// - when the remote namespace previously created has been correctly removed from this cluster.
		// In these cases the remote condition will be "OffloadingRequired=false"
		assignClusterRemoteCondition(noff, "", nml.Items[i].GetName())
	}
}

// setNamespaceOffloadingStatus sets global offloading status according to the remote namespace conditions.
func setNamespaceOffloadingStatus(noff *offv1alpha1.NamespaceOffloading) {
	ready := 0
	notReady := 0

	for i := range noff.Status.RemoteNamespacesConditions {
		condition := liqoutils.FindRemoteNamespaceCondition(noff.Status.RemoteNamespacesConditions[i], offv1alpha1.NamespaceReady)
		if condition == nil {
			continue
		}
		if condition.Status == corev1.ConditionTrue {
			ready++
		} else {
			notReady++
		}
	}

	switch {
	case !noff.DeletionTimestamp.IsZero():
		noff.Status.OffloadingPhase = offv1alpha1.TerminatingOffloadingPhaseType
		if ready+notReady == 0 {
			// The NamespaceOffloading is deleted only when there are no more remoteNamespaceCondition and
			// the deletion timestamp is set.
			for key := range noff.Status.RemoteNamespacesConditions {
				delete(noff.Status.RemoteNamespacesConditions, key)
			}
			klog.Infof("NamespaceOffloading, in the namespace '%s', ready to be deleted", noff.Namespace)
		}
	case ready+notReady == 0:
		noff.Status.OffloadingPhase = offv1alpha1.NoClusterSelectedOffloadingPhaseType
	case ready == 0:
		noff.Status.OffloadingPhase = offv1alpha1.AllFailedOffloadingPhaseType
	case notReady == 0:
		noff.Status.OffloadingPhase = offv1alpha1.ReadyOffloadingPhaseType
	default:
		noff.Status.OffloadingPhase = offv1alpha1.SomeFailedOffloadingPhaseType
	}

	klog.Infof("The OffloadingStatus for the NamespaceOffloading in the namespace '%s' is set to '%s'",
		noff.Namespace, noff.Status.OffloadingPhase)
}

// ensureRemoteConditionsConsistence checks for every remote condition of the NamespaceOffloading resource that the
// corresponding NamespaceMap is still there. If the peering is deleted also the corresponding remote condition
// must be deleted.
func ensureRemoteConditionsConsistence(noff *offv1alpha1.NamespaceOffloading, nml *mapsv1alpha1.NamespaceMapList) {
	for nmname := range noff.Status.RemoteNamespacesConditions {
		presence := false
		for i := range nml.Items {
			if nml.Items[i].GetName() == nmname {
				presence = true
				break
			}
		}
		if !presence {
			delete(noff.Status.RemoteNamespacesConditions, nmname)
			klog.Infof("The remoteNamespaceCondition for the remote cluster '%s' is no more necessary", nmname)
		}
	}
}
