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
	"k8s.io/apimachinery/pkg/runtime/schema"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
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

const (
	// ResourceSliceClassUnknown is the unknown class of the ResourceSlice.
	ResourceSliceClassUnknown ResourceSliceClass = ""
	// ResourceSliceClassDefault is the default class of the ResourceSlice.
	ResourceSliceClassDefault ResourceSliceClass = "default"
)

// ResourceSliceSpec defines the desired state of ResourceSlice.
type ResourceSliceSpec struct {
	// ConsumerClusterID is the id of the consumer cluster.
	ConsumerClusterID *liqov1beta1.ClusterID `json:"consumerClusterID,omitempty"`
	// ProviderClusterID is the id of the provider cluster.
	ProviderClusterID *liqov1beta1.ClusterID `json:"providerClusterID,omitempty"`
	// Resources contains the slice of resources requested.
	Resources corev1.ResourceList `json:"resources,omitempty"`
	// Class contains the class of the ResourceSlice.
	Class ResourceSliceClass `json:"class,omitempty"`
	// CSR is the Certificate Signing Request of the consumer cluster.
	CSR []byte `json:"csr,omitempty"`
}

// ResourceSliceConditionType represents different types of conditions that a ResourceSlice could assume.
type ResourceSliceConditionType string

const (
	// ResourceSliceConditionTypeAuthentication informs users that the authentication of the ResourceSlice is in progress.
	ResourceSliceConditionTypeAuthentication ResourceSliceConditionType = "Authentication"
	// ResourceSliceConditionTypeResources informs users that the resources of the ResourceSlice are in progress.
	ResourceSliceConditionTypeResources ResourceSliceConditionType = "Resources"
)

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
	// Type of the condition.
	// +kubebuilder:validation:Enum="Authentication";"Resources"
	Type ResourceSliceConditionType `json:"type"`
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
	// Conditions contains the conditions of the ResourceSlice.
	Conditions []ResourceSliceCondition `json:"conditions,omitempty"`
	// Resources contains the slice of resources accepted.
	Resources corev1.ResourceList `json:"resources,omitempty"`
	// AuthParams contains the authentication parameters for the resources given by the provider cluster.
	AuthParams *AuthParams `json:"authParams,omitempty"`
	// StorageClasses contains the list of the storage classes offered by the cluster.
	StorageClasses []liqov1beta1.StorageType `json:"storageClasses,omitempty"`
	// IngressClasses contains the list of the ingress classes offered by the cluster.
	IngressClasses []liqov1beta1.IngressType `json:"ingressClasses,omitempty"`
	// LoadBalancerClasses contains the list of the load balancer classes offered by the cluster.
	LoadBalancerClasses []liqov1beta1.LoadBalancerType `json:"loadBalancerClasses,omitempty"`
	// NodeLabels contains the provider cluster labels.
	NodeLabels map[string]string `json:"nodeLabels,omitempty"`
	// NodeSelector contains the selector to be applied to offloaded pods.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=rslice
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Authentication",type=string,JSONPath=`.status.conditions[?(@.type=="Authentication")].status`
// +kubebuilder:printcolumn:name="Resources",type=string,JSONPath=`.status.conditions[?(@.type=="Resources")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ResourceSlice represents a slice of resources given by the provider cluster to the consumer cluster.
type ResourceSlice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceSliceSpec   `json:"spec,omitempty"`
	Status ResourceSliceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceSliceList contains a list of ResourceSlices.
type ResourceSliceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceSlice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceSlice{}, &ResourceSliceList{})
}
