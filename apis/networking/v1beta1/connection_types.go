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

// ConnectionStatusValue represents the status of a connection.
type ConnectionStatusValue string

const (
	// ConnectionTypeServer represents a server connection.
	ConnectionTypeServer ConnectionType = "Server"
	// ConnectionTypeClient represents a client connection.
	ConnectionTypeClient ConnectionType = "Client"

	// Connected used when the connection is up and running.
	Connected ConnectionStatusValue = "Connected"
	// Connecting used as temporary status while waiting for the vpn tunnel to come up.
	Connecting ConnectionStatusValue = "Connecting"
	// ConnectionError used to se the status in case of errors.
	ConnectionError ConnectionStatusValue = "Error"
)

// ConnectionSpec defines the desired state of Connection.
type ConnectionSpec struct {
	// Type of the connection.
	// +kubebuilder:validation:Enum=Server;Client
	Type ConnectionType `json:"type"`
	// GatewayRef specifies the reference to the gateway.
	GatewayRef corev1.ObjectReference `json:"gatewayRef"`
}

// ConnectionLatency represents the latency between two clusters.
type ConnectionLatency struct {
	// Value of the latency.
	Value string `json:"value,omitempty"`
	// Timestamp of the latency.
	Timestamp metav1.Time `json:"timestamp,omitempty"`
}

// ConnectionStatus defines the observed state of Connection.
type ConnectionStatus struct {
	// Value of the connection.
	Value ConnectionStatusValue `json:"value,omitempty"`
	// Latency of the connection.
	Latency ConnectionLatency `json:"latency,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=conn
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.value`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Latency",type=string,JSONPath=`.status.latency.value`,priority=1

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
