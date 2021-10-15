// Copyright 2019-2021 The Liqo Authors
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
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ShadowPodSpec defines the desired state of ShadowPod.
type ShadowPodSpec struct {
	// Pod contains the complete Kubernetes pod specification.
	// +kubebuilder:validation:EmbeddedResource
	Pod runtime.RawExtension `json:"pod"`
}

// ShadowPodStatus defines the observed state of ShadowPod.
type ShadowPodStatus struct {
}

// +kubebuilder:object:root=true

// ShadowPod is the Schema for the Shadowpods API.
type ShadowPod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ShadowPodSpec   `json:"spec,omitempty"`
	Status ShadowPodStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ShadowPodList contains a list of ShadowPod.
type ShadowPodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShadowPod `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ShadowPod{}, &ShadowPodList{})
}
