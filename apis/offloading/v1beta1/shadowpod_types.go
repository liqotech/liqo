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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ShadowPodSpec defines the desired state of ShadowPod.
type ShadowPodSpec struct {
	Pod corev1.PodSpec `json:"pod,omitempty"`
}

// ShadowPodStatus defines the observed state of ShadowPod.
type ShadowPodStatus struct {
	// Phase is the status of this ShadowPod.
	// When the pod is created it is checked by the operator, which sets this field same as pod status.
	// +kubebuilder:validation:Enum="Pending";"Running";"Succeeded";"Failed";"Unknown"
	// +kubebuilder:default="Unknown"
	Phase corev1.PodPhase `json:"phase"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=shp;shpod
// +kubebuilder:subresource:status
// +genclient
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

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
