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

package consts

const (
	// RemoteClusterID is used to obtain cluster-id from different Liqo resources.
	RemoteClusterID = "liqo.io/remote-cluster-id"
	// TypeLabel is the key of a Liqo label that identifies different types of nodes.
	// todo: change to NodeTypeLabel
	TypeLabel = "liqo.io/type"
	// TypeNode is the value of a Liqo label that identifies Liqo virtual nodes.
	// todo: change to VirtualNodeType
	TypeNode = "virtual-node"
	// DocumentationURL is the URL to official Liqo Documentation.
	DocumentationURL = "https://doc.liqo.io/"
	// DefaultNamespaceOffloadingName is the default name of NamespaceOffloading resources. Every namespace that has
	// to be offloaded with Liqo, must have a NamespaceOffloading resource with this name.
	DefaultNamespaceOffloadingName = "offloading"
	// SchedulingLiqoLabel is necessary in order to allow Pods to be scheduled on remote clusters.
	SchedulingLiqoLabel = "liqo.io/scheduling-enabled"
	// SchedulingLiqoLabelValue unique value allowed for SchedulingLiqoLabel.
	SchedulingLiqoLabelValue = "true"

	// RemoteNamespaceManagedByAnnotationKey is the annotation that identifies the NamespaceMap managing a given remote namespace.
	RemoteNamespaceManagedByAnnotationKey = "liqo.io/managed-by-namespace-map"
	// RemoteNamespaceOriginalNameAnnotationKey is the annotation that identifies the original name of a remote namespace.
	RemoteNamespaceOriginalNameAnnotationKey = "liqo.io/original-name"
	// RemoteNamespaceClusterRoleName is the name of the cluster role used to grant permissions to the virtual kubelet in remote namespaces.
	RemoteNamespaceClusterRoleName = "liqo-virtual-kubelet-remote"
)
