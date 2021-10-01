// Copyright 2019-2021 The Liqo Authors
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

package virtualKubelet

const (
	// VirtualNodePrefix -> the prefix used to generate the virtual node name.
	VirtualNodePrefix = "liqo-"
	// VirtualKubeletPrefix -> the prefix used to generate the virtual kubelet deployment name.
	VirtualKubeletPrefix = "virtual-kubelet-"
	// ReflectedpodKey -> the key of the label added to reflected pods.
	ReflectedpodKey = "virtualkubelet.liqo.io/source-pod"
	// HomePodFinalizer -> the finalizer added to local pods when reflected.
	HomePodFinalizer = "virtual-kubelet.liqo.io/provider"

	// Clients configuration.
	HOME_CLIENT_QPS      = 1000
	HOME_CLIENTS_BURST   = 5000
	FOREIGN_CLIENT_QPS   = 1000
	FOREIGN_CLIENT_BURST = 5000
)

// VirtualNodeName generates the virtual node name based on the cluster ID.
func VirtualNodeName(clusterID string) string {
	return VirtualNodePrefix + clusterID
}
