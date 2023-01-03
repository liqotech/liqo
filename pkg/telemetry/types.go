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

package telemetry

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
)

// NamespaceInfo contains information about an offloaded namespace.
type NamespaceInfo struct {
	UID                string                                          `json:"uid,omitempty"`
	MappingStrategy    offloadingv1alpha1.NamespaceMappingStrategyType `json:"mappingStrategy,omitempty"`
	OffloadingStrategy offloadingv1alpha1.PodOffloadingStrategyType    `json:"offloadingStrategy,omitempty"`
	HasClusterSelector bool                                            `json:"hasClusterSelector,omitempty"`
	NumOffloadedPods   map[string]int64                                `json:"numOffloadedPods,omitempty"`
}

// PeeringDetails contains information about a peering direction.
type PeeringDetails struct {
	Enabled   bool                `json:"enabled"`
	Resources corev1.ResourceList `json:"resources,omitempty"`
}

// PeeringInfo contains information about a peering.
type PeeringInfo struct {
	RemoteClusterID string                        `json:"remoteClusterID"`
	Method          discoveryv1alpha1.PeeringType `json:"method,omitempty"`
	DiscoveryType   discovery.Type                `json:"discoveryType,omitempty"`
	Latency         time.Duration                 `json:"latency,omitempty"`
	Incoming        PeeringDetails                `json:"incoming"`
	Outgoing        PeeringDetails                `json:"outgoing"`
}

// Telemetry contains information about the cluster.
type Telemetry struct {
	ClusterID         string          `json:"clusterID"`
	LiqoVersion       string          `json:"liqoVersion,omitempty"`
	KubernetesVersion string          `json:"kubernetesVersion,omitempty"`
	Provider          string          `json:"provider,omitempty"`
	PeeringInfo       []PeeringInfo   `json:"peeringInfo,omitempty"`
	NamespacesInfo    []NamespaceInfo `json:"namespacesInfo,omitempty"`
}

// Builder is the constructor for the Telemetry struct.
type Builder struct {
	Client            client.Client
	Namespace         string
	LiqoVersion       string
	KubernetesVersion string
	ClusterLabels     map[string]string
}
