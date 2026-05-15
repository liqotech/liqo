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

// FirewallConfigurationBindingResource is the name of the FirewallConfigurationBinding resources.
// firewallconfigurationbindings is used for resource name pluralization because the k8s API does not manage false friends.
// Waiting for this fix https://github.com/kubernetes-sigs/kubebuilder/pull/3408
var FirewallConfigurationBindingResource = "firewallconfigurationbindings"

// FirewallConfigurationBindingKind is the kind name used to register the FirewallConfigurationBinding CRD.
var FirewallConfigurationBindingKind = "FirewallConfigurationBinding"

// FirewallConfigurationBindingGroupResource is group resource used to register these objects.
var FirewallConfigurationBindingGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: FirewallConfigurationBindingResource}

// FirewallConfigurationBindingGroupVersionResource is groupResourceVersion used to register these objects.
var FirewallConfigurationBindingGroupVersionResource = GroupVersion.WithResource(FirewallConfigurationBindingResource)

// TargetReference is a typed object reference identifying the entity responsible for
// applying a FirewallConfigurationBinding (e.g. a gateway pod or an InternalNode).
// It carries a full GroupVersionKind so that the garbage collector can resolve and observe
// the target generically, without hardcoding per-kind logic.
type TargetReference struct {
	// APIVersion of the referenced target (e.g. "v1" for a Pod or
	// "networking.liqo.io/v1beta1" for an InternalNode).
	APIVersion string `json:"apiVersion"`
	// Kind of the referenced target (e.g. "Pod" or "InternalNode").
	Kind string `json:"kind"`
	// Name of the referenced target.
	Name string `json:"name"`
	// Namespace of the referenced target. It must be empty for cluster-scoped targets
	// (e.g. an InternalNode) and set for namespaced targets (e.g. a Pod).
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// FirewallConfigurationBindingSpec defines the desired state of FirewallConfigurationBinding.
type FirewallConfigurationBindingSpec struct {
	// FirewallConfigurationRef is the reference to the FirewallConfiguration to apply.
	FirewallConfigurationRef corev1.LocalObjectReference `json:"firewallConfigurationRef"`
	// TargetRef identifies the entity (e.g. a gateway pod or an InternalNode) that should apply this binding.
	// The FirewallConfigurationBinding controller running on the matching entity filters resources by this field.
	// It must be unique across all FirewallConfigurationBinding controllers instances in the cluster,
	// otherwise multiple entities will apply the same FirewallConfiguration, which may cause unexpected behavior.
	TargetRef TargetReference `json:"targetRef"`
}

// FirewallConfigurationBindingConditionType is a type of FirewallConfigurationBinding condition.
type FirewallConfigurationBindingConditionType string

const (
	// FirewallConfigurationBindingConditionTypeApplied is true if the configuration has been applied.
	FirewallConfigurationBindingConditionTypeApplied FirewallConfigurationBindingConditionType = "Applied"
)

// FirewallConfigurationBindingStatus defines the observed state of FirewallConfigurationBinding.
type FirewallConfigurationBindingStatus struct {
	// Conditions contains the conditions of the FirewallConfigurationBinding.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// TableName is the name of the nftables table managed by this binding.
	// Cached here so that cleanup can proceed even after the FirewallConfiguration is deleted.
	TableName string `json:"tableName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,path=firewallconfigurationbindings,shortName=fwbinding;fwcfgbinding
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Applied",type=string,JSONPath=`.status.conditions[?(@.type=='Applied')].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="FirewallConfiguration",type=string,JSONPath=`.spec.firewallConfigurationRef.name`,priority=1
// +kubebuilder:printcolumn:name="TargetKind",type=string,JSONPath=`.spec.targetRef.kind`,priority=1
// +kubebuilder:printcolumn:name="TargetName",type=string,JSONPath=`.spec.targetRef.name`,priority=1
// +kubebuilder:printcolumn:name="TargetNamespace",type=string,JSONPath=`.spec.targetRef.namespace`,priority=1

// FirewallConfigurationBinding links an entity (e.g. a fabric pod or gateway) to a FirewallConfiguration.
// The entity that owns this resource is responsible for applying the referenced FirewallConfiguration
// and for cleaning up the nftables rules when this resource is deleted.
type FirewallConfigurationBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FirewallConfigurationBindingSpec   `json:"spec,omitempty"`
	Status FirewallConfigurationBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FirewallConfigurationBindingList contains a list of FirewallConfigurationBinding.
type FirewallConfigurationBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FirewallConfigurationBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FirewallConfigurationBinding{}, &FirewallConfigurationBindingList{})
}
