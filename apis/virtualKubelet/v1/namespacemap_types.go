/*


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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NamespaceMapSpec defines the desired state of NamespaceMap
type NamespaceMapSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ATTENZIONE POTREBBE ESSERE 30 LA maxLength (da ricontrollare!!)
	// +kubebuilder:validation:MaxLength=50
	// +kubebuilder:validation:MinLength=1
	RemoteClusterId string `json:"remoteClusterId"`
}

// NamespaceMapStatus defines the observed state of NamespaceMap
type NamespaceMapStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	NattingTable   map[string]string `json:"nattingTable,omitempty"`
	DeNattingTable map[string]string `json:"deNattingTable,omitempty"`
}

// +kubebuilder:object:root=true

// NamespaceMap is the Schema for the namespacemaps API
type NamespaceMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespaceMapSpec   `json:"spec,omitempty"`
	Status NamespaceMapStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NamespaceMapList contains a list of NamespaceMap
type NamespaceMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespaceMap `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NamespaceMap{}, &NamespaceMapList{})
}
