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

// IsNetworkingModuleDisabled checks if the liqo networking module is disabled.
func IsNetworkingModuleDisabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return !foreignCluster.Status.Modules.Networking.Enabled
}

// IsAuthenticationModuleDisabled checks if the liqo authentication module is disabled.
func IsAuthenticationModuleDisabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return !foreignCluster.Status.Modules.Authentication.Enabled
}

// IsOffloadingModuleDisabled checks if the liqo offloading module is disabled.
func IsOffloadingModuleDisabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return !foreignCluster.Status.Modules.Offloading.Enabled
}

// IsNetworkingEstablished checks if the networking is established.
func IsNetworkingEstablished(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := GetStatus(foreignCluster.Status.Modules.Networking.Conditions, discoveryv1alpha1.NetworkConnectionStatusCondition)
	return curPhase == discoveryv1alpha1.ConditionStatusEstablished
}

// IsAuthenticationEstablished checks if the authentication is established.
func IsAuthenticationEstablished(_ *discoveryv1alpha1.ForeignCluster) bool {
	// TODO: implement the function
	return true
}

// IsOffloadingEstablished checks if the offloading is established.
func IsOffloadingEstablished(_ *discoveryv1alpha1.ForeignCluster) bool {
	// TODO: implement the function
	return true
}

// IsNetworkingEstablishedOrDisabled checks if the networking is established or if the liqo networking module is disabled.
func IsNetworkingEstablishedOrDisabled(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	return IsNetworkingEstablished(foreignCluster) || IsNetworkingModuleDisabled(foreignCluster)
}

// IsAPIServerReady checks if the api server is ready.
func IsAPIServerReady(foreignCluster *discoveryv1alpha1.ForeignCluster) bool {
	curPhase := GetStatus(foreignCluster.Status.Modules.Offloading.Conditions, discoveryv1alpha1.OffloadingAPIServerStatusCondition)
	return curPhase == discoveryv1alpha1.ConditionStatusEstablished
}
