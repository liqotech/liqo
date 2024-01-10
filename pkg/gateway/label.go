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

package gateway

const (
	// GatewayComponentKey is the key used to label the gateway pod.
	GatewayComponentKey = "networking.liqo.io/component"

	// GatewayComponentGateway is the key used to label the gateway pod.
	GatewayComponentGateway = "gateway"
)

// ForgeGatewayPodLabels returns the labels for the gateway pod.
func ForgeGatewayPodLabels() map[string]string {
	return map[string]string{
		GatewayComponentKey: GatewayComponentGateway,
	}
}
