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

package virtualKubelet

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
)

const (
	// VirtualNodePrefix -> the prefix used to generate the virtual node name.
	VirtualNodePrefix = "liqo-"
)

// VirtualNodeGroupName generate the group name for the virtual nodes referring a specific clusterID.
func VirtualNodesGroupName(cluster *discoveryv1alpha1.ClusterIdentity) string {
	return VirtualNodePrefix + cluster.ClusterName
}

// VirtualNodeName generate the name for a virtual node.
func VirtualNodeName(vn *virtualkubeletv1alpha1.VirtualNode) string {
	return VirtualNodePrefix + vn.Spec.ClusterIdentity.ClusterName + "-" + vn.Name
}
