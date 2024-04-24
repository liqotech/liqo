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
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

// IsAuthenticated checks if the identity has been accepted by the remote cluster.
func IsAuthenticated(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.AuthenticationStatusCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusEstablished
}

// IsIncomingJoined checks if the incoming peering has been completely established.
func IsIncomingJoined(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.IncomingPeeringCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusEstablished
}

// IsOutgoingJoined checks if the outgoing peering has been completely established.
func IsOutgoingJoined(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.OutgoingPeeringCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusEstablished
}

// IsIncomingEnabled checks if the incoming peering is enabled (i.e. Pending, Established or Deleting).
func IsIncomingEnabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.IncomingPeeringCondition)
	return curPhase != discoveryv1alpha1.PeeringConditionStatusNone && curPhase != ""
}

// IsOutgoingEnabled checks if the outgoing peering is enabled (i.e. Pending, Established or Deleting).
func IsOutgoingEnabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.OutgoingPeeringCondition)
	return curPhase != discoveryv1alpha1.PeeringConditionStatusNone && curPhase != ""
}

// IsIncomingPeeringNone checks if the incoming peering is set to none.
func IsIncomingPeeringNone(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.IncomingPeeringCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusNone
}

// IsIncomingPeeringYes checks if the incoming peering is set to Yes.
func IsIncomingPeeringYes(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return foreignCluster.Spec.IncomingPeeringEnabled == discoveryv1alpha1.PeeringEnabledYes
}

// IsIncomingPeeringNo checks if the incoming peering is set to No.
func IsIncomingPeeringNo(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return foreignCluster.Spec.IncomingPeeringEnabled == discoveryv1alpha1.PeeringEnabledNo
}

// IsOutgoingPeeringNone checks if the outgoing peering is set to none.
func IsOutgoingPeeringNone(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.OutgoingPeeringCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusNone
}

// IsUnpeered returns whether the no peering is currently active towards the remote cluster.
func IsUnpeered(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return IsIncomingPeeringNone(foreignCluster) && IsOutgoingPeeringNone(foreignCluster)
}

// IsNetworkingEstablished checks if the networking has be established.
func IsNetworkingEstablished(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.NetworkStatusCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusEstablished
}

// IsNetworkingDisabled checks if the liqo networking module is disabled.
func IsNetworkingDisabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.NetworkStatusCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusDisabled
}

// IsNetworkingEstablishedOrDisabled checks if the networking has be established or if the liqo networking module is disabled.
func IsNetworkingEstablishedOrDisabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return IsNetworkingEstablished(foreignCluster) || IsNetworkingDisabled(foreignCluster)
}

// IsAPIServerReady checks if the api server is ready.
func IsAPIServerReady(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := peeringconditionsutils.GetStatus(foreignCluster, discoveryv1alpha1.APIServerStatusCondition)
	return curPhase == discoveryv1alpha1.PeeringConditionStatusEstablished
}
