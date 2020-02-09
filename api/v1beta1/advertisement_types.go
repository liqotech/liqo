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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AdvertisementSpec defines the desired state of Advertisement
type AdvertisementSpec struct {
	ClusterId    string                  `json:"clusterId"`
	Images       []corev1.ContainerImage `json:"images"`
	Availability corev1.ResourceList     `json:"availability"`
	Prices       corev1.ResourceList     `json:"prices"`
	Timestamp    metav1.Time             `json:"timestamp"`
	Validity     metav1.Time             `json:"validity"`
}

// AdvertisementStatus defines the observed state of Advertisement
type AdvertisementStatus struct {
	Phase string `json:"phase"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName="adv"

// Advertisement is the Schema for the advertisements API
type Advertisement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AdvertisementSpec   `json:"spec,omitempty"`
	Status AdvertisementStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AdvertisementList contains a list of Advertisement
type AdvertisementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Advertisement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Advertisement{}, &AdvertisementList{})
}
