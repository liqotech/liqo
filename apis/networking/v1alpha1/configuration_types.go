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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ConfigurationResource the name of the configuration resources.
var ConfigurationResource = "configurations"

// ConfigurationKind is the kind name used to register the Configuration CRD.
var ConfigurationKind = "Configuration"

// ConfigurationGroupResource is group resource used to register these objects.
var ConfigurationGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: ConfigurationResource}

// ConfigurationGroupVersionResource is groupResourceVersion used to register these objects.
var ConfigurationGroupVersionResource = GroupVersion.WithResource(ConfigurationResource)

// CIDR defines the CIDR of the cluster.
type CIDR struct {
	// Pod CIDR of the cluster.
	Pod string `json:"pod,omitempty"`
	// External CIDR of the cluster.
	External string `json:"external,omitempty"`
}

// ClusterConfig defines the configuration of a cluster.
type ClusterConfig struct {
	// CIDR of the cluster.
	CIDR CIDR `json:"cidr,omitempty"`
}

// ConfigurationSpec defines the desired state of Configuration.
type ConfigurationSpec struct {
	// Local network configuration (the cluster where the resource is created).
	Local ClusterConfig `json:"local,omitempty"`
	// Remote network configuration (the other cluster).
	Remote ClusterConfig `json:"remote,omitempty"`
}

// ConfigurationStatus defines the observed state of Configuration.
type ConfigurationStatus struct {
	// Remote remapped configuration, it defines how the local cluster sees the remote cluster.
	Remote *ClusterConfig `json:"remote,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status

// Configuration contains the network configuration of a pair of clusters,
// including the local and the remote pod and external CIDRs and how the where remapped.
type Configuration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurationSpec   `json:"spec,omitempty"`
	Status ConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigurationList contains a list of Configuration.
type ConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Configuration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Configuration{}, &ConfigurationList{})
}
