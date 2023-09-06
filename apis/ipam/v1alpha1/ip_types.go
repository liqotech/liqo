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

var (
	// IPKind is the kind name used to register the IP CRD.
	IPKind = "IP"

	// IPResource is the resource name used to register the IP CRD.
	IPResource = "ips"

	// IPGroupVersionResource is the group version resource used to register IP CRD.
	IPGroupVersionResource = GroupVersion.WithResource(IPResource)

	// IPGroupResource is the group resource used to register IP CRD.
	IPGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: IPResource}
)

// IPSpec defines a local IP.
type IPSpec struct {
	// IP is the local IP.
	IP string `json:"ip"`
}

// IPStatus defines remapped IPs.
type IPStatus struct {
	// IPMappings contains the mapping of the local IP for each remote cluster.
	IPMappings map[string]string `json:"ipMappings,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Local IP",type=string,JSONPath=`.spec.ip`
// +kubebuilder:printcolumn:name="Remapped IPs",type=string,JSONPath=`.status.ipMappings`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// IP is the Schema for the IP API.
type IP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPSpec   `json:"spec"`
	Status IPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPList contains a list of IP.
type IPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IP{}, &IPList{})
}
