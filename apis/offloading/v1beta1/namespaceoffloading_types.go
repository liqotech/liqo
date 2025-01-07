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

// OffloadingPhaseType represents different namespaces offloading status.
type OffloadingPhaseType string

const (
	// ReadyOffloadingPhaseType -> remote Namespaces have been correctly created on previously selected clusters.
	ReadyOffloadingPhaseType OffloadingPhaseType = "Ready"
	// NoClusterSelectedOffloadingPhaseType -> no cluster matches user constraints.
	NoClusterSelectedOffloadingPhaseType OffloadingPhaseType = "NoClusterSelected"
	// InProgressOffloadingPhaseType -> remote Namespaces' creation is still ongoing.
	InProgressOffloadingPhaseType OffloadingPhaseType = "InProgress"
	// SomeFailedOffloadingPhaseType -> there was an error during creation of some remote Namespaces.
	SomeFailedOffloadingPhaseType OffloadingPhaseType = "SomeFailed"
	// AllFailedOffloadingPhaseType -> there was an error during creation of all remote Namespaces.
	AllFailedOffloadingPhaseType OffloadingPhaseType = "AllFailed"
	// TerminatingOffloadingPhaseType -> means remote namespaces are undergoing graceful termination.
	TerminatingOffloadingPhaseType OffloadingPhaseType = "Terminating"
)

// NamespaceMappingStrategyType represents different strategies to map local and remote namespace names.
type NamespaceMappingStrategyType string

const (
	// EnforceSameNameMappingStrategyType -> the remote namespace is assigned the same name of the local one
	// (the creation may fail in case of conflicts).
	EnforceSameNameMappingStrategyType NamespaceMappingStrategyType = "EnforceSameName"
	// DefaultNameMappingStrategyType -> the remote namespace is assigned a default name which ensures uniqueness
	// and avoids conflicts (localNamespaceName-localClusterID).
	DefaultNameMappingStrategyType NamespaceMappingStrategyType = "DefaultName"
	// SelectedNameMappingStrategyType -> the remote namespace is assigned a name chosen by the user.
	// (the creation may fail in case of conflicts).
	SelectedNameMappingStrategyType NamespaceMappingStrategyType = "SelectedName"
)

// PodOffloadingStrategyType represents different strategies to offload pods in this Namespace.
type PodOffloadingStrategyType string

const (
	// LocalPodOffloadingStrategyType -> the pods in this namespace can be scheduled on the local cluster only
	// (i.e. no pod offloading occurs).
	LocalPodOffloadingStrategyType PodOffloadingStrategyType = "Local"
	// RemotePodOffloadingStrategyType -> the pods in this namespace can be scheduled on remote clusters only, possibly
	// filtered through the ClusterSelector field.
	RemotePodOffloadingStrategyType PodOffloadingStrategyType = "Remote"
	// LocalAndRemotePodOffloadingStrategyType -> the pods in this namespace can be scheduled on both the local
	// and remote clusters, the latter possibly filtered through the ClusterSelector field.
	LocalAndRemotePodOffloadingStrategyType PodOffloadingStrategyType = "LocalAndRemote"
)

// RemoteNamespaceConditionType represents different conditions that a remote namespace could assume.
type RemoteNamespaceConditionType string

// These are valid conditions of a remote namespace.
const (
	// NamespaceOffloadingRequired, informs users if their namespace has been offloaded on this cluster or not.
	NamespaceOffloadingRequired RemoteNamespaceConditionType = "OffloadingRequired"
	// NamespaceReady, remote Namespace is correctly created and ready to be used.
	NamespaceReady RemoteNamespaceConditionType = "Ready"
)

// RemoteNamespaceConditions list of RemoteNamespaceCondition.
type RemoteNamespaceConditions []RemoteNamespaceCondition

// RemoteNamespaceCondition contains details about state of remote namespace.
type RemoteNamespaceCondition struct {
	// Type of remote namespace controller condition.
	Type RemoteNamespaceConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// LastTransitionTime -> timestamp for when the Namespace last transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason -> Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message -> Human-readable message indicating details about the last status transition.
	Message string `json:"message,omitempty"`
}

// NamespaceOffloadingSpec defines the desired state of NamespaceOffloading.
type NamespaceOffloadingSpec struct {
	//  NamespaceMappingStrategy allows users to map local and remote namespace names according to two
	//  different strategies: "DefaultName", which ensures uniqueness and prevents conflicts, and "EnforceSameName",
	//  which enforces the same name at the cost of possible conflicts.
	// +kubebuilder:validation:Enum="EnforceSameName";"DefaultName";"SelectedName"
	// +kubebuilder:default="DefaultName"
	// +kubebuilder:validation:Optional
	NamespaceMappingStrategy NamespaceMappingStrategyType `json:"namespaceMappingStrategy"`

	// RemoteNamespaceName allows users to choose a specific name for the remote namespace.
	// This field is required if NamespaceMappingStrategy is set to "SelectedName". It is ignored otherwise.
	RemoteNamespaceName string `json:"remoteNamespaceName,omitempty"`

	// PodOffloadingStrategy allows users to configure how pods in this namespace are offloaded, according to three
	// different strategies: "Local" (i.e. no pod offloading is performed), "Remote" (i.e. all pods are offloaded
	// in remote clusters), "LocalAndRemote" (i.e. no constraints are enforced besides the ones
	// specified by the ClusterSelector).
	// +kubebuilder:validation:Enum="Local";"Remote";"LocalAndRemote"
	// +kubebuilder:default="LocalAndRemote"
	// +kubebuilder:validation:Optional
	PodOffloadingStrategy PodOffloadingStrategyType `json:"podOffloadingStrategy"`

	// ClusterSelector allows users to select a specific subset of remote clusters to perform
	// pod offloading by means of the standard Kubernetes NodeSelector approach
	// (https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity).
	// A cluster selector with no NodeSelectorTerms matches all clusters.
	ClusterSelector corev1.NodeSelector `json:"clusterSelector,omitempty"`
}

// NamespaceOffloadingStatus defines the observed state of NamespaceOffloading.
type NamespaceOffloadingStatus struct {
	// RemoteNamespaceName is the remote namespace name chosen by means of the NamespaceMappingStrategy.
	RemoteNamespaceName string `json:"remoteNamespaceName,omitempty"`
	// OffloadingPhase -> informs users about namespaces offloading status:
	// "Ready" (i.e. remote Namespaces have been correctly created on previously selected clusters.)
	// "NoClusterSelected" (i.e. no cluster matches user constraints.)
	// "InProgress" (i.e. remote Namespaces' creation is still ongoing.)
	// "SomeFailed" (i.e. there was an error during creation of some remote Namespaces.)
	// "AllFailed" (i.e. there was an error during creation of all remote Namespaces.)
	// "Terminating" (i.e. remote namespaces are undergoing graceful termination.)
	OffloadingPhase OffloadingPhaseType `json:"offloadingPhase,omitempty"`
	// RemoteNamespacesConditions -> allows user to verify remote Namespaces' presence and status on all remote
	// clusters through RemoteNamespaceCondition.
	RemoteNamespacesConditions map[string]RemoteNamespaceConditions `json:"remoteNamespacesConditions,omitempty"`
	// The generation observed by the NamespaceOffloading controller.
	// This field allows external tools (e.g., liqoctl) to detect whether a spec modification has already been processed
	// or not (i.e., whether the status should be expected to be up-to-date or not), and thus act accordingly.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=nso;nsof;nsoff;nsoffloading
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="NamespaceMappingStrategy",type=string,JSONPath=`.spec.namespaceMappingStrategy`
// +kubebuilder:printcolumn:name="PodOffloadingStrategy",type=string,JSONPath=`.spec.podOffloadingStrategy`
// +kubebuilder:printcolumn:name="OffloadingPhase",type=string,JSONPath=`.status.offloadingPhase`
// +kubebuilder:printcolumn:name="RemoteNamespaceName",type=string,JSONPath=`.status.remoteNamespaceName`,priority=10
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// NamespaceOffloading is the Schema for the namespaceoffloadings API.
type NamespaceOffloading struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespaceOffloadingSpec   `json:"spec"`
	Status NamespaceOffloadingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NamespaceOffloadingList contains a list of NamespaceOffloading.
type NamespaceOffloadingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespaceOffloading `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NamespaceOffloading{}, &NamespaceOffloadingList{})
}
