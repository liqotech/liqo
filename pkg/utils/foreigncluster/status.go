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
)

// IsNetworkingEstablished checks if the networking is established.
func IsNetworkingEstablished(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := GetStatus(foreignCluster.Status.Modules.Networking.Conditions, discoveryv1alpha1.NetworkConnectionStatusCondition)
	return curPhase == discoveryv1alpha1.ConditionStatusEstablished
}

// IsNetworkingEstablishedOrDisabled checks if the networking is established or if the liqo networking module is disabled.
func IsNetworkingEstablishedOrDisabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return IsNetworkingEstablished(foreignCluster) || !IsNetworkingModuleEnabled(foreignCluster)
}

// GetAPIServerStatus returns the status of the api server.
func GetAPIServerStatus(foreignCluster *discoveryv1alpha1.ForeignCluster) discoveryv1alpha1.ConditionStatusType {
	return GetStatus(foreignCluster.Status.Conditions, discoveryv1alpha1.APIServerStatusCondition)
}

// IsAPIServerReadyOrDisabled checks if the api server is ready or not applicable.
func IsAPIServerReadyOrDisabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := GetAPIServerStatus(foreignCluster)
	return curPhase == discoveryv1alpha1.ConditionStatusEstablished || curPhase == discoveryv1alpha1.ConditionStatusNone
}
