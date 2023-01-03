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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	discv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ResourceTunnelEndpoints the name of the tunnelendpoint resources.
var ResourceTunnelEndpoints = "tunnelendpoints"

// TunnelEndpointSpec defines the desired state of TunnelEndpoint.
type TunnelEndpointSpec struct {
	// The identity of the remote cluster.
	ClusterIdentity discv1alpha1.ClusterIdentity `json:"clusterIdentity"`

	// PodCIDR of local cluster.
	LocalPodCIDR string `json:"localPodCIDR"`
	// Network used in the remote cluster to map the local PodCIDR, in case of conflicts (in the remote cluster).
	// +kubebuilder:default="None"
	// +kubebuilder:validation:Optional
	LocalNATPodCIDR string `json:"localNATPodCIDR"`
	// ExternalCIDR of local cluster.
	LocalExternalCIDR string `json:"localExternalCIDR"`
	// Network used in the remote cluster to map the local ExternalCIDR, in case of conflicts (in the remote cluster).
	// +kubebuilder:default="None"
	// +kubebuilder:validation:Optional
	LocalNATExternalCIDR string `json:"localNATExternalCIDR"`

	// PodCIDR of remote cluster.
	RemotePodCIDR string `json:"remotePodCIDR"`
	// Network used in the local cluster to map the remote cluster PodCIDR, in case of conflicts with RemotePodCIDR.
	// +kubebuilder:default="None"
	// +kubebuilder:validation:Optional
	RemoteNATPodCIDR string `json:"remoteNATPodCIDR"`
	// ExternalCIDR of remote cluster.
	RemoteExternalCIDR string `json:"remoteExternalCIDR"`
	// Network used in the local cluster to map the remote cluster ExternalCIDR, in case of conflicts with RemoteExternalCIDR.
	// +kubebuilder:default="None"
	// +kubebuilder:validation:Optional
	RemoteNATExternalCIDR string `json:"remoteNATExternalCIDR"`

	// Public IP of the node where the VPN tunnel is created.
	EndpointIP string `json:"endpointIP"`
	// Vpn technology used to interconnect two clusters.
	BackendType string `json:"backendType"`
	// Connection parameters.
	BackendConfig map[string]string `json:"backend_config"`
}

// TunnelEndpointStatus defines the observed state of TunnelEndpoint.
type TunnelEndpointStatus struct {
	TunnelIFaceIndex int        `json:"tunnelIFaceIndex,omitempty"`
	TunnelIFaceName  string     `json:"tunnelIFaceName,omitempty"`
	VethIFaceIndex   int        `json:"vethIFaceIndex,omitempty"`
	VethIFaceName    string     `json:"vethIFaceName,omitempty"`
	VethIP           string     `json:"vethIP,omitempty"`
	GatewayIP        string     `json:"gatewayIP,omitempty"`
	Connection       Connection `json:"connection,omitempty"`
}

// ConnectionLatency represents the latency between two clusters.
type ConnectionLatency struct {
	Value     string      `json:"value,omitempty"`
	Timestamp metav1.Time `json:"timestamp,omitempty"`
}

// Connection holds the configuration and status of a vpn tunnel connecting to remote cluster.
type Connection struct {
	Status            ConnectionStatus  `json:"status,omitempty"`
	StatusMessage     string            `json:"statusMessage,omitempty"`
	PeerConfiguration map[string]string `json:"peerConfiguration,omitempty"`
	Latency           ConnectionLatency `json:"latency,omitempty"`
}

// ConnectionStatus type that describes the status of vpn connection with a remote cluster.
type ConnectionStatus string

const (
	// Connected used when the connection is up and running.
	Connected ConnectionStatus = "Connected"
	// ConnectedMessage human-readable info when the vpn connection is in "connected" state.
	ConnectedMessage string = "VPN connection established"
	// Connecting used as temporary status while waiting for the vpn tunnel to come up.
	Connecting ConnectionStatus = "Connecting"
	// ConnectingMessage human-readable info when the vpn is in "connecting" state.
	ConnectingMessage string = "Waiting VPN connection to be established"
	// ConnectionError used to se the status in case of errors.
	ConnectionError ConnectionStatus = "Error"
	// ConnectionErrorMessage is the message sent when a connection error occurs.
	ConnectionErrorMessage string = "No network connectivity towards remote cluster"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status

// TunnelEndpoint is the Schema for the endpoints API.
// +kubebuilder:printcolumn:name="Peering Cluster",type=string,JSONPath=`.spec.clusterIdentity.clusterName`
// +kubebuilder:printcolumn:name="Endpoint IP",type=string,JSONPath=`.spec.endpointIP`,priority=1
// +kubebuilder:printcolumn:name="Backend type",type=string,JSONPath=`.spec.backendType`
// +kubebuilder:printcolumn:name="Latency",type=string,JSONPath=`.status.connection.latency.value`,priority=1
// +kubebuilder:printcolumn:name="Connection status",type=string,JSONPath=`.status.connection.status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type TunnelEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TunnelEndpointSpec   `json:"spec,omitempty"`
	Status TunnelEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TunnelEndpointList contains a list of TunnelEndpoint.
type TunnelEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TunnelEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TunnelEndpoint{}, &TunnelEndpointList{})
}
