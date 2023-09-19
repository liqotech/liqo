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

	v1alpha1networking "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

var (
	// NetworkKind is the kind name used to register the Network CRD.
	NetworkKind = "Network"

	// NetworkResource is the resource name used to register the Network CRD.
	NetworkResource = "networks"

	// NetworkGroupVersionResource is the group version resource used to register the Network CRD.
	NetworkGroupVersionResource = GroupVersion.WithResource(NetworkResource)

	// NetworkGroupResource is the group resource used to register the Network CRD.
	NetworkGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: NetworkResource}
)

// NetworkSpec defines the desired state of Network.
type NetworkSpec struct {
	// CIDR is the desired CIDR for the remote cluster.
	CIDR v1alpha1networking.CIDR `json:"cidr"`
}

// NetworkStatus defines the observed state of Network.
type NetworkStatus struct {
	// CIDR is the remapped CIDR for the remote cluster.
	CIDR v1alpha1networking.CIDR `json:"cidr,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Desired CIDR",type=string,JSONPath=`.spec.cidr`
// +kubebuilder:printcolumn:name="Remapped CIDR",type=string,JSONPath=`.status.cidr`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Network is the Schema for the Network API.
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec"`
	Status NetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkList contains a list of Network.
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Network{}, &NetworkList{})
}
