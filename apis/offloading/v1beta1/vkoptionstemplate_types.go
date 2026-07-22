// Copyright 2019-2026 The Liqo Authors
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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// VkOptionsTemplateSpec defines the desired state of VkOptionsTemplate.
type VkOptionsTemplateSpec struct {
	CreateNode              bool                            `json:"createNode"`
	DisableNetworkCheck     bool                            `json:"disableNetworkCheck"`
	ContainerImage          string                          `json:"containerImage"`
	MetricsEnabled          bool                            `json:"metricsEnabled"`
	MetricsAddress          string                          `json:"metricsAddress,omitempty"`
	LabelsNotReflected      []string                        `json:"labelsNotReflected,omitempty"`
	AnnotationsNotReflected []string                        `json:"annotationsNotReflected,omitempty"`
	ReflectorsConfig        map[string]ReflectorConfig      `json:"reflectorsConfig,omitempty"`
	CustomResources         []CustomResourceReflectorConfig `json:"customResources,omitempty"`
	Resources               corev1.ResourceRequirements     `json:"resources,omitempty"`
	ExtraArgs               []string                        `json:"extraArgs,omitempty"`
	ExtraAnnotations        map[string]string               `json:"extraAnnotations,omitempty"`
	ExtraLabels             map[string]string               `json:"extraLabels,omitempty"`
	NodeExtraAnnotations    map[string]string               `json:"nodeExtraAnnotations,omitempty"`
	NodeExtraLabels         map[string]string               `json:"nodeExtraLabels,omitempty"`
	Replicas                *int32                          `json:"replicas,omitempty"`
	ImagePullSecrets        []corev1.LocalObjectReference   `json:"imagePullSecrets,omitempty"`
	PullPolicy              corev1.PullPolicy               `json:"pullPolicy,omitempty"`
	Tolerations             []corev1.Toleration             `json:"tolerations,omitempty"`
}

// ReflectorConfig contains configuration parameters of the reflector.
type ReflectorConfig struct {
	// Number of workers for the reflector.
	NumWorkers uint `json:"workers"`
	// Type of reflection.
	Type ReflectionType `json:"type,omitempty"`
}

// CustomResourceReflectorConfig contains configuration parameters for reflecting a custom resource GVR.
type CustomResourceReflectorConfig struct {
	// Group is the API group of the custom resource (e.g., "example.io").
	Group string `json:"group"`
	// Version is the API version of the custom resource (e.g., "v1").
	Version string `json:"version"`
	// Resource is the plural resource name of the custom resource (e.g., "widgets").
	Resource string `json:"resource"`
	// ReflectorConfig contains the common reflector configuration (workers, type).
	ReflectorConfig `json:",inline"`
}

// GVR returns the GroupVersionResource for this custom resource configuration.
func (c CustomResourceReflectorConfig) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    c.Group,
		Version:  c.Version,
		Resource: c.Resource,
	}
}

// ReflectionType is the type of reflection.
type ReflectionType string

const (
	// AllowList reflects only the resources with a specific annotation.
	AllowList ReflectionType = "AllowList"
	// DenyList reflects all the resources excluding the ones with a specific annotation.
	DenyList ReflectionType = "DenyList"
	// CustomLiqo reflects the resources following the custom Liqo logic.
	CustomLiqo ReflectionType = "CustomLiqo"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=vkot;vkopt
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +genclient

// VkOptionsTemplate is the Schema with the options to configure the VirtualKubelet deployment.
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
