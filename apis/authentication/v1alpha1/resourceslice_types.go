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
	"k8s.io/apimachinery/pkg/runtime/schema"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// ResourceSliceResource is the name of the resourceSlice resources.
var ResourceSliceResource = "resourceslices"

// ResourceSliceKind specifies the kind of the resourceSlice.
var ResourceSliceKind = "ResourceSlice"

// ResourceSliceGroupResource is group resource used to register these objects.
var ResourceSliceGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: ResourceSliceResource}

// ResourceSliceGroupVersionResource is groupResourceVersion used to register these objects.
var ResourceSliceGroupVersionResource = GroupVersion.WithResource(ResourceSliceResource)

// ResourceSliceClass is the class of the ResourceSlice.
type ResourceSliceClass string

// ResourceSliceSpec defines the desired state of ResourceSlice.
type ResourceSliceSpec struct {
	// ConsumerClusterIdentity is the identity of the consumer cluster.
	ConsumerClusterIdentity discoveryv1alpha1.ClusterIdentity `json:"consumerClusterIdentity,omitempty"`
	// ProviderClusterIdentity is the identity of the provider cluster.
	ProviderClusterIdentity discoveryv1alpha1.ClusterIdentity `json:"providerClusterIdentity,omitempty"`
	// Resources contains the slice of resources requested.
	Resources corev1.ResourceList `json:"resources,omitempty"`
	// Class contains the class of the ResourceSlice.
	Class ResourceSliceClass `json:"class,omitempty"`
	// CSR is the Certificate Signing Request of the consumer cluster.
	CSR string `json:"csr,omitempty"`
}

// ResourceSliceConditionStatus represents different status conditions that a ResourceSlice could assume.
type ResourceSliceConditionStatus string

// These are valid conditions of a ResourceSlice.
const (
	// ResourceSliceConditionAccepted informs users that the resources are available.
	ResourceSliceConditionAccepted ResourceSliceConditionStatus = "Accepted"
	// ResourceSliceConditionDenied informs users that the resources are not available.
	ResourceSliceConditionDenied ResourceSliceConditionStatus = "Denied"
)

// ResourceSliceCondition contains details about the status of the provided ResourceSlice.
type ResourceSliceCondition struct {
	// Status of the condition.
	// +kubebuilder:validation:Enum="Accepted";"Denied"
	Status ResourceSliceConditionStatus `json:"status"`
	// LastTransitionTime -> timestamp for when the condition last transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason -> Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message -> Human-readable message indicating details about the last status transition.
	Message string `json:"message,omitempty"`
}

// ResourceSliceStatus defines the observed state of ResourceSlice.
type ResourceSliceStatus struct {
	// Condition contains the condition of the ResourceSlice.
	Condition ResourceSliceCondition `json:"condition,omitempty"`
	// Resources contains the slice of resources accepted.
	Resources corev1.ResourceList `json:"resources,omitempty"`
	// AuthParams contains the authentication parameters for the resources given by the provider cluster.
	AuthParams AuthParams `json:"authParams,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ResourceSlice represents a slice of resources given by the provider cluster to the consumer cluster.
type ResourceSlice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceSliceSpec   `json:"spec,omitempty"`
	Status ResourceSliceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceSliceList contains a list of Identities.
type ResourceSliceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceSlice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceSlice{}, &ResourceSliceList{})
}
