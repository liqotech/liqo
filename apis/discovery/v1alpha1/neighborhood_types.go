/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type Neighbor struct{}

// NeighborhoodSpec defines the desired state of Neighborhood
type NeighborhoodSpec struct {
	// ClusterID is the ID of the sender of this resource.
	ClusterID string `json:"clusterID"`
	// NeighborsList contains the clusters that have peered with the local cluster.
	NeighborsList map[string]Neighbor `json:"neighborsList"`
}

// NeighborhoodStatus defines the observed state of Neighborhood
type NeighborhoodStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Neighborhood is the Schema for the neighborhoods API
// +kubebuilder:printcolumn:name="Local",type=string,JSONPath=`.metadata.labels.liqo\.io/replication`
type Neighborhood struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NeighborhoodSpec   `json:"spec,omitempty"`
	Status NeighborhoodStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NeighborhoodList contains a list of Neighborhood
type NeighborhoodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Neighborhood `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Neighborhood{}, &NeighborhoodList{})
}
