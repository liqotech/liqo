// Copyright 2019-2024 The Liqo Authors
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VkOptionsTemplateSpec defines the desired state of VkOptionsTemplate.
type VkOptionsTemplateSpec struct {
	CreateNode              bool                        `json:"createNode"`
	DisableNetworkCheck     bool                        `json:"disableNetworkCheck"`
	ContainerImage          string                      `json:"containerImage"`
	MetricsEnabled          bool                        `json:"metricsEnabled"`
	MetricsAddress          string                      `json:"metricsAddress,omitempty"`
	LabelsNotReflected      []string                    `json:"labelsNotReflected,omitempty"`
	AnnotationsNotReflected []string                    `json:"annotationsNotReflected,omitempty"`
	ReflectorsWorkers       map[string]uint             `json:"reflectorsWorkers"`
	ReflectorsType          map[string]string           `json:"reflectorsType"`
	Resources               corev1.ResourceRequirements `json:"resources,omitempty"`
	ExtraArgs               []string                    `json:"extraArgs,omitempty"`
	ExtraAnnotations        map[string]string           `json:"extraAnnotations,omitempty"`
	ExtraLabels             map[string]string           `json:"extraLabels,omitempty"`
	NodeExtraAnnotations    map[string]string           `json:"nodeExtraAnnotations,omitempty"`
	NodeExtraLabels         map[string]string           `json:"nodeExtraLabels,omitempty"`
}

// +kubebuilder:object:root=true
// +genclient

// VkOptionsTemplate is the Schema with the options to configure the VirtualKubelet deployment.
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type VkOptionsTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VkOptionsTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// VkOptionsTemplateList contains a list of VkOptionsTemplate.
type VkOptionsTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VkOptionsTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VkOptionsTemplate{}, &VkOptionsTemplateList{})
}
