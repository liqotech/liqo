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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mappingPhase string

const (
	MappingAccepted mappingPhase = "Accepted"
	MappingRefused  mappingPhase = "Refused"
)

type RemoteNamespaceStatus struct {
	// RemoteNamespace is the name chosen by the user at creation time (when he puts labels on his local namespace)
	RemoteNamespace string `json:"remoteNamespace,omitempty"`
	// Phase is the remote Namespace's actual status (Accepted,Refused)
	Phase mappingPhase `json:"phase,omitempty"`
}

// NamespaceMapSpec defines the desired state of NamespaceMap
type NamespaceMapSpec struct {

	// DesiredMapping is filled by NamespaceController when a user requires to offload a remote namespace, every entry
	// of the map represents the localNamespaceName[key]-remoteNamespaceName[value] association. When a new entry is
	// created the NamespaceMap Controller tries to create the associated remote namespace.
	DesiredMapping map[string]string `json:"desiredMapping,omitempty"`
}

// NamespaceMapStatus defines the observed state of NamespaceMap
type NamespaceMapStatus struct {

	// CurrentMapping is filled by NamespaceMap Controller, when a new remote namespace's creation is requested. The key
	// is the local namespace name, while the value is a summary of new remote namespace's status
	CurrentMapping map[string]RemoteNamespaceStatus `json:"currentMapping,omitempty"`
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
