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

// ConnectionResource the name of the connection resources.
var ConnectionResource = "connections"

// ConnectionKind specifies the kind of the connection.
var ConnectionKind = "Connection"

// ConnectionGroupResource is group resource used to register these objects.
var ConnectionGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: ConnectionResource}

// ConnectionGroupVersionResource is groupResourceVersion used to register these objects.
var ConnectionGroupVersionResource = GroupVersion.WithResource(ConnectionResource)

// ConnectionType represents the type of a connection.
type ConnectionType string

const (
	// ConnectionTypeServer represents a server connection.
	ConnectionTypeServer ConnectionType = "Server"
	// ConnectionTypeClient represents a client connection.
	ConnectionTypeClient ConnectionType = "Client"
)

// PingSpec defines the desired state of Ping.
type PingSpec struct {
	// Enabled specifies whether the ping is enabled or not.
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`
	// Endpoint specifies the endpoint to ping.
	Endpoint EndpointStatus `json:"endpoint,omitempty"`
}

// ConnectionSpec defines the desired state of Connection.
type ConnectionSpec struct {
	// Type of the connection.
	// +kubebuilder:validation:Enum=Server;Client
	Type ConnectionType `json:"type"`
	// GatewayRef specifies the reference to the gateway.
	GatewayRef corev1.ObjectReference `json:"gatewayRef"`
	// Ping specifies the ping configuration.
	Ping PingSpec `json:"ping,omitempty"`
}

// ConnectionConditionType represents different conditions that a connection could assume.
type ConnectionConditionType string

const (
	// ConnectionConditionEstablished represents a connection that is established.
	ConnectionConditionEstablished ConnectionConditionType = "Established"
	// ConnectionConditionPending represents a connection that is pending.
	ConnectionConditionPending ConnectionConditionType = "Pending"
	// ConnectionConditionDenied represents a connection that is denied.
	ConnectionConditionDenied ConnectionConditionType = "Denied"
	// ConnectionConditionError represents a connection that is in error.
	ConnectionConditionError ConnectionConditionType = "Error"
)

// ConnectionConditionStatusType represents the status of a connection condition.
type ConnectionConditionStatusType string

const (
	// ConnectionConditionStatusTrue represents a connection condition that is true.
	ConnectionConditionStatusTrue ConnectionConditionStatusType = "True"
	// ConnectionConditionStatusFalse represents a connection condition that is false.
	ConnectionConditionStatusFalse ConnectionConditionStatusType = "False"
	// ConnectionConditionStatusUnknown represents a connection condition that is unknown.
	ConnectionConditionStatusUnknown ConnectionConditionStatusType = "Unknown"
)

// ConnectionCondition contains details about state of the connection.
type ConnectionCondition struct {
	// Type of the connection condition.
	// +kubebuilder:validation:Enum="Established"
	Type ConnectionConditionType `json:"type"`
	// Status of the condition.
	// +kubebuilder:validation:Enum="True";"False";"Unknown"
	// +kubebuilder:default="Unknown"
	Status ConnectionConditionStatusType `json:"status"`
	// LastTransitionTime -> timestamp for when the condition last transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason -> Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message -> Human-readable message indicating details about the last status transition.
	Message string `json:"message,omitempty"`
}

// ConnectionStatus defines the observed state of Connection.
type ConnectionStatus struct {
	// Conditions contains the conditions of the connection.
	Conditions []ConnectionCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status

// Connection contains the status of a connection between two clusters (a client and a server).
type Connection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConnectionSpec   `json:"spec,omitempty"`
	Status ConnectionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConnectionList contains a list of Connection.
type ConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Connection `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Connection{}, &ConnectionList{})
}
