// Copyright 2019-2022 The Liqo Authors
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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SearchDomainSpec defines the desired state of SearchDomain.
type SearchDomainSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// DNS domain where to search for subscribed remote clusters
	Domain string `json:"domain"`
	// Enable join process for retrieved clusters
	AutoJoin bool `json:"autojoin"`
}

// SearchDomainStatus defines the observed state of SearchDomain.
type SearchDomainStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// SearchDomain is the Schema for the SearchDomains API.
type SearchDomain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SearchDomainSpec   `json:"spec,omitempty"`
	Status SearchDomainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SearchDomainList contains a list of SearchDomain.
type SearchDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SearchDomain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SearchDomain{}, &SearchDomainList{})
}
