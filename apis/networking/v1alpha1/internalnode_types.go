// Copyright 2019-2023 The Liqo Authors
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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// InternalNodeResource the name of the internalNode resources.
var InternalNodeResource = "internalnodes"

// InternalNodeKind is the kind name used to register the InternalNode CRD.
var InternalNodeKind = "InternalNode"

// InternalNodeGroupResource is group resource used to register these objects.
var InternalNodeGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: InternalNodeResource}

// InternalNodeGroupVersionResource is groupResourceVersion used to register these objects.
var InternalNodeGroupVersionResource = GroupVersion.WithResource(InternalNodeResource)

// InternalNodeSpec defines the desired state of InternalNode.
type InternalNodeSpec struct {
	// IP is the IP address to assign to the internal interface.
	IP IP `json:"ip,omitempty"`
	// NodeAddr is the address of the node.
	NodeAddr string `json:"nodeAddr,omitempty"`
}

// InternalNodeConditionType is a valid value for InternalNodeCondition.Type.
type InternalNodeConditionType string

const (
	// InternalNodeConditionTypeApplied means the route has been applied.
	InternalNodeConditionTypeApplied InternalNodeConditionType = "Applied"
)

// InternalNodeCondition contains details for the current condition of this node.
type InternalNodeCondition struct {
	// Type is the type of the condition.
	// +kubebuilder:validation:Enum=Applied
	Type InternalNodeConditionType `json:"type"`
	// Status is the status of the condition.
	// +kubebuilder:validation:Enum=True;False;Unknown
	// +kubebuilder:default=Unknown
	Status corev1.ConditionStatus `json:"status"`
	// LastTransitionTime is the last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message is a human-readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

// InternalNodeStatus defines the observed state of InternalNode.
type InternalNodeStatus struct {
	// Conditions contains information about the current status of the node.
	Conditions []InternalNodeCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=liqo
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type == 'Applied')].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// InternalNode contains the network internalnode of a pair of clusters,
// including the local and the remote pod and external CIDRs and how the where remapped.
type InternalNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InternalNodeSpec   `json:"spec,omitempty"`
	Status InternalNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InternalNodeList contains a list of InternalNode.
type InternalNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InternalNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InternalNode{}, &InternalNodeList{})
}
