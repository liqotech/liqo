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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RouteConfigurationBindingResource is the plural resource name for RouteConfigurationBinding.
var RouteConfigurationBindingResource = "routeconfigurationbindings"

// RouteConfigurationBindingKind is the kind name used to register the RouteConfigurationBinding CRD.
var RouteConfigurationBindingKind = "RouteConfigurationBinding"

// RouteConfigurationBindingGroupResource is group resource used to register these objects.
var RouteConfigurationBindingGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: RouteConfigurationBindingResource}

// RouteConfigurationBindingGroupVersionResource is groupResourceVersion used to register these objects.
var RouteConfigurationBindingGroupVersionResource = GroupVersion.WithResource(RouteConfigurationBindingResource)

// RouteBindingTargetReference is a typed object reference identifying the entity responsible for
// applying a RouteConfigurationBinding (e.g. a gateway pod or a fabric node).
// It carries a full GroupVersionKind so that the garbage collector can resolve and observe
// the target generically, without hardcoding per-kind logic.
type RouteBindingTargetReference struct {
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

// String returns a human-readable representation of the target reference.
func (t RouteBindingTargetReference) String() string {
	if t.Namespace == "" {
		return fmt.Sprintf("%s/%s %s", t.APIVersion, t.Kind, t.Name)
	}
	return fmt.Sprintf("%s/%s %s/%s", t.APIVersion, t.Kind, t.Namespace, t.Name)
}

// RouteConfigurationBindingSpec defines the desired state of RouteConfigurationBinding.
type RouteConfigurationBindingSpec struct {
	// RouteConfigurationRef is the reference to the RouteConfiguration to apply.
	RouteConfigurationRef corev1.LocalObjectReference `json:"routeConfigurationRef"`
	// TargetRef identifies the entity (for example, a gateway pod or Liqo node fabric)
	// to which this RouteConfigurationBinding binds a RouteConfiguration.
	//
	// A RouteConfigurationBinding controller only reconciles bindings whose
	// TargetRef matches the target it manages. For a given target, there must be
	// exactly one controller instance in the cluster. Otherwise, multiple
	// controllers will apply the referenced RouteConfiguration, resulting in
	// unexpected behavior.
	TargetRef RouteBindingTargetReference `json:"targetRef"`
}

// RouteConfigurationBindingConditionType is a type of RouteConfigurationBinding condition.
type RouteConfigurationBindingConditionType string

const (
	// RouteConfigurationBindingConditionTypeApplied is true if the configuration has been applied.
	RouteConfigurationBindingConditionTypeApplied RouteConfigurationBindingConditionType = "Applied"
)

// RouteConfigurationBindingConditionReason is a reason for a RouteConfigurationBinding condition.
type RouteConfigurationBindingConditionReason string

const (
	// RouteConfigurationBindingConditionReasonApplySucceeded indicates the configuration was applied successfully.
	RouteConfigurationBindingConditionReasonApplySucceeded RouteConfigurationBindingConditionReason = "ApplySucceeded"
	// RouteConfigurationBindingConditionReasonApplyFailed indicates the configuration could not be applied.
	RouteConfigurationBindingConditionReasonApplyFailed RouteConfigurationBindingConditionReason = "ApplyFailed"
)

// RouteConfigurationBindingStatus defines the observed state of RouteConfigurationBinding.
type RouteConfigurationBindingStatus struct {
	// Conditions contains the conditions of the RouteConfigurationBinding.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// TableName is the name of the routing table managed by this binding.
	// Cached here so that cleanup can proceed even after the RouteConfiguration is deleted.
	TableName string `json:"tableName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,path=routeconfigurationbindings,shortName=routebinding;routecfgbinding
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Applied",type=string,JSONPath=`.status.conditions[?(@.type=='Applied')].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="RouteConfiguration",type=string,JSONPath=`.spec.routeConfigurationRef.name`,priority=1
// +kubebuilder:printcolumn:name="TargetKind",type=string,JSONPath=`.spec.targetRef.kind`,priority=1
// +kubebuilder:printcolumn:name="TargetName",type=string,JSONPath=`.spec.targetRef.name`,priority=1
// +kubebuilder:printcolumn:name="TargetNamespace",type=string,JSONPath=`.spec.targetRef.namespace`,priority=1

// RouteConfigurationBinding links an entity (e.g. a fabric pod or gateway) to a RouteConfiguration.
// The entity that owns this resource is responsible for applying the referenced RouteConfiguration
// and for cleaning up the routes when this resource is deleted.
type RouteConfigurationBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteConfigurationBindingSpec   `json:"spec,omitempty"`
	Status RouteConfigurationBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RouteConfigurationBindingList contains a list of RouteConfigurationBinding.
type RouteConfigurationBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RouteConfigurationBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RouteConfigurationBinding{}, &RouteConfigurationBindingList{})
}
