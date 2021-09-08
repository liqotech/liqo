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

package consts

const (
	// RemoteClusterID is used to obtain cluster-id from different Liqo resources.
	RemoteClusterID = "cluster-id" // "remote.liqo.io/clusterId"
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
	// EnablingLiqoLabel is used to create a default NamespaceOffloading resource for the labeled namespace, this
	// is an alternative way to start Liqo offloading.
	EnablingLiqoLabel = "liqo.io/enabled"
	// EnablingLiqoLabelValue unique value allowed for EnablingLiqoLabel.
	EnablingLiqoLabelValue = "true"
	// SchedulingLiqoLabel is necessary in order to allow Pods to be scheduled on remote clusters.
	SchedulingLiqoLabel = "liqo.io/scheduling-enabled"
	// SchedulingLiqoLabelValue unique value allowed for SchedulingLiqoLabel.
	SchedulingLiqoLabelValue = "true"
	// RoleBindingLabelKey label that some RoleBindings in the remote namespace must have. In every remote namespace
	// there are some RoleBindings that provide the local virtualKubelet with some privileges. These RoleBindings just
	// described must have that RoleBindingLabel.
	RoleBindingLabelKey = "capsule.clastix.io/tenant"
	// RoleBindingLabelValuePrefix prefix of the value that the RoleBindingLabel must have.
	RoleBindingLabelValuePrefix = "tenant"
	// RemoteNamespaceAnnotationKey is the annotation that all remote namespaces created by the NamespaceMap controller
	// must have.
	RemoteNamespaceAnnotationKey = "liqo.io/remote-namespace"
)
