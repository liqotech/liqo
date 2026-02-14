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

// Package directconnection contains utility functions and types used when direct connection between providers is requested.
package directconnection

import (
	corev1 "k8s.io/api/core/v1"
)

// ShouldIncludeDataFromNode returns whether to include the direct connection data
// (IP and clusterID) of the pods deployed on this node to the remote cluster.
//
// It returns false in case the node is not virtual and in case it's not the one this VK is reflecting to.
//
// Used only when the the use-direct-connections is requested.
//
// E.G.: in case this VK is reflecting to "clusterA", no data from pods running on nodes belonging to "clusterA" will be included.
func ShouldIncludeDataFromNode(node *corev1.Node, remoteClusterID string) bool {
	if node == nil {
		return false
	}

	if node.Labels == nil {
		return false
	}

	if node.Labels["liqo.io/type"] != "virtual-node" {
		return false
	}

	if node.Labels["liqo.io/remote-cluster-id"] == remoteClusterID {
		return false
	}

	return true
}
