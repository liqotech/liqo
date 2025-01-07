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

package v1beta1

import (
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EndpointSliceTemplate defines the desired state of the EndpointSlice.
type EndpointSliceTemplate struct {
	Endpoints   []discoveryv1.Endpoint     `json:"endpoints,omitempty"`
	Ports       []discoveryv1.EndpointPort `json:"ports,omitempty"`
	AddressType discoveryv1.AddressType    `json:"addressType,omitempty"`
}

// ShadowEndpointSliceSpec defines the desired state of ShadowEndpointSlice.
type ShadowEndpointSliceSpec struct {
	Template EndpointSliceTemplate `json:"template,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=shes;sheps;seps
// +genclient

// ShadowEndpointSlice is the Schema for the ShadowEndpointSlices API.
type ShadowEndpointSlice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ShadowEndpointSliceSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ShadowEndpointSliceList contains a list of ShadowEndpointSlice.
type ShadowEndpointSliceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShadowEndpointSlice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ShadowEndpointSlice{}, &ShadowEndpointSliceList{})
}
