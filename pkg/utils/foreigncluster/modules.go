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

package foreigncluster

import liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"

// EnableModuleNetworking enables the networking module.
func EnableModuleNetworking(foreignCluster *liqov1beta1.ForeignCluster) {
	foreignCluster.Status.Modules.Networking.Enabled = true
}

// EnableModuleAuthentication enables the authentication module.
func EnableModuleAuthentication(foreignCluster *liqov1beta1.ForeignCluster) {
	foreignCluster.Status.Modules.Authentication.Enabled = true
}

// EnableModuleOffloading enables the offloading module.
func EnableModuleOffloading(foreignCluster *liqov1beta1.ForeignCluster) {
	foreignCluster.Status.Modules.Offloading.Enabled = true
}

// DisableModuleNetworking disables the networking module.
func DisableModuleNetworking(foreignCluster *liqov1beta1.ForeignCluster) {
	foreignCluster.Status.Modules.Networking.Enabled = false
}

// DisableModuleAuthentication disables the authentication module.
func DisableModuleAuthentication(foreignCluster *liqov1beta1.ForeignCluster) {
	foreignCluster.Status.Modules.Authentication.Enabled = false
}

// DisableModuleOffloading disables the offloading module.
func DisableModuleOffloading(foreignCluster *liqov1beta1.ForeignCluster) {
	foreignCluster.Status.Modules.Offloading.Enabled = false
}

// IsNetworkingModuleEnabled checks if the networking module is enabled.
func IsNetworkingModuleEnabled(foreignCluster *liqov1beta1.ForeignCluster) bool {
	return foreignCluster.Status.Modules.Networking.Enabled
}

// IsAuthenticationModuleEnabled checks if the authentication module is enabled.
func IsAuthenticationModuleEnabled(foreignCluster *liqov1beta1.ForeignCluster) bool {
	return foreignCluster.Status.Modules.Authentication.Enabled
}

// IsOffloadingModuleEnabled checks if the offloading module is enabled.
func IsOffloadingModuleEnabled(foreignCluster *liqov1beta1.ForeignCluster) bool {
	return foreignCluster.Status.Modules.Offloading.Enabled
}
