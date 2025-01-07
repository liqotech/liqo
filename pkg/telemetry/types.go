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

package telemetry

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
)

// NodeInfo contains information about a node.
type NodeInfo struct {
	KernelVersion string `json:"kernelVersion,omitempty"`
	OsImage       string `json:"osImage,omitempty"`
	Architecture  string `json:"architecture,omitempty"`
}

// NamespaceInfo contains information about an offloaded namespace.
type NamespaceInfo struct {
	UID                string                                         `json:"uid,omitempty"`
	MappingStrategy    offloadingv1beta1.NamespaceMappingStrategyType `json:"mappingStrategy,omitempty"`
	OffloadingStrategy offloadingv1beta1.PodOffloadingStrategyType    `json:"offloadingStrategy,omitempty"`
	HasClusterSelector bool                                           `json:"hasClusterSelector,omitempty"`
	NumOffloadedPods   map[string]int64                               `json:"numOffloadedPods,omitempty"`
}

// PeeringDetails contains information about a peering direction.
type PeeringDetails struct {
	Enabled   bool                `json:"enabled"`
	Resources corev1.ResourceList `json:"resources,omitempty"`
}

// ModuleInfo contains information about a module.
type ModuleInfo struct {
	Enabled bool `json:"enabled"`
}

// ModulesInfo contains information about the modules.
type ModulesInfo struct {
	Networking     ModuleInfo `json:"networking"`
	Authentication ModuleInfo `json:"authentication"`
	Offloading     ModuleInfo `json:"offloading"`
}

// PeeringInfo contains information about a peering.
type PeeringInfo struct {
	RemoteClusterID     liqov1beta1.ClusterID `json:"remoteClusterID"`
	Modules             ModulesInfo           `json:"modules,omitempty"`
	Role                liqov1beta1.RoleType  `json:"role,omitempty"`
	Latency             time.Duration         `json:"latency,omitempty"`
	NodesNumber         int                   `json:"nodesNumber"`
	VirtualNodesNumber  int                   `json:"virtualNodesNumber"`
	ResourceSliceNumber int                   `json:"resourceSliceNumber"`
}

// Telemetry contains information about the cluster.
type Telemetry struct {
	ClusterID         string              `json:"clusterID"`
	LiqoVersion       string              `json:"liqoVersion,omitempty"`
	KubernetesVersion string              `json:"kubernetesVersion,omitempty"`
	NodesInfo         map[string]NodeInfo `json:"nodesInfo,omitempty"`
	Provider          string              `json:"provider,omitempty"`
	PeeringInfo       []PeeringInfo       `json:"peeringInfo,omitempty"`
	NamespacesInfo    []NamespaceInfo     `json:"namespacesInfo,omitempty"`
}

// Builder is the constructor for the Telemetry struct.
type Builder struct {
	Client            client.Client
	Namespace         string
	LiqoVersion       string
	KubernetesVersion string
	ClusterLabels     map[string]string
}
