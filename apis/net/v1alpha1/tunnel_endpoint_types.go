// Copyright 2019-2021 The Liqo Authors
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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TunnelEndpointSpec defines the desired state of TunnelEndpoint.
type TunnelEndpointSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The ID of the remote cluster that will receive this CRD.
	ClusterID string `json:"clusterID"`
	// PodCIDR of remote cluster.
	PodCIDR string `json:"podCIDR"`
	// ExternalCIDR of remote cluster.
	ExternalCIDR string `json:"externalCIDR"`
	// Public IP of the node where the VPN tunnel is created.
	EndpointIP string `json:"endpointIP"`
	// Vpn technology used to interconnect two clusters.
	BackendType string `json:"backendType"`
	// Connection parameters.
	BackendConfig map[string]string `json:"backend_config"`
}

// TunnelEndpointStatus defines the observed state of TunnelEndpoint.
type TunnelEndpointStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file\

	Phase string `json:"phase,omitempty"`
	// PodCIDR of local cluster.
	LocalPodCIDR string `json:"localPodCIDR,omitempty"`
	// Network used in the remote cluster to map the local PodCIDR, in case of conflicts(in the remote cluster).
	// Default is "None".
	LocalNATPodCIDR string `json:"localNATPodCIDR,omitempty"`
	// Network used in the local cluster to map the remote cluster PodCIDR, in case of conflicts with Spec.PodCIDR.
	// Default is "None".
	RemoteNATPodCIDR string `json:"remoteNATPodCIDR,omitempty"`
	// ExternalCIDR of local cluster.
	LocalExternalCIDR string `json:"localExternalCIDR,omitempty"`
	// Network used in the remote cluster to map the local ExternalCIDR, in case of conflicts(in the remote cluster).
	// Default is "None".
	LocalNATExternalCIDR string `json:"localNATExternalCIDR,omitempty"`
	// Network used in the local cluster to map the remote cluster ExternalCIDR, in case of conflicts with
	// Spec.ExternalCIDR.
	// Default is "None".
	RemoteNATExternalCIDR string     `json:"remoteNATExternalCIDR,omitempty"`
	RemoteEndpointIP      string     `json:"remoteTunnelPublicIP,omitempty"`
	LocalEndpointIP       string     `json:"localTunnelPublicIP,omitempty"`
	TunnelIFaceIndex      int        `json:"tunnelIFaceIndex,omitempty"`
	TunnelIFaceName       string     `json:"tunnelIFaceName,omitempty"`
	VethIFaceIndex        int        `json:"vethIFaceIndex,omitempty"`
	VethIFaceName         string     `json:"vethIFaceName,omitempty"`
	GatewayIP             string     `json:"gatewayIP,omitempty"`
	Connection            Connection `json:"connection,omitempty"`
}

// Connection holds the configuration and status of a vpn tunnel connecting to remote cluster.
type Connection struct {
	Status            ConnectionStatus  `json:"status,omitempty"`
	StatusMessage     string            `json:"statusMessage,omitempty"`
	PeerConfiguration map[string]string `json:"peerConfiguration,omitempty"`
}

// ConnectionStatus type that describes the status of vpn connection with a remote cluster.
type ConnectionStatus string

const (
	// Connected used when the connection is up and running.
	Connected ConnectionStatus = "connected"
	// Connecting used as temporary status while waiting for the vpn tunnel to come up.
	Connecting ConnectionStatus = "connecting"
	// ConnectionError used to se the status in case of errors.
	ConnectionError ConnectionStatus = "error"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TunnelEndpoint is the Schema for the endpoints API.
// +kubebuilder:printcolumn:name="Peering Cluster ID",type=string,JSONPath=`.spec.clusterID`
// +kubebuilder:printcolumn:name="Endpoint IP",type=string,JSONPath=`.spec.endpointIP`,priority=1
// +kubebuilder:printcolumn:name="Backend type",type=string,JSONPath=`.spec.backendType`
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
