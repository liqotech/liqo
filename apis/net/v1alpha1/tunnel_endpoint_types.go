/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TunnelEndpointSpec defines the desired state of TunnelEndpoint
type TunnelEndpointSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	//the ID of the remote cluster that will receive this CRD
	ClusterID string `json:"clusterID"`
	//network subnet used in the local cluster for the pod IPs
	PodCIDR string `json:"podCIDR"`
	//public IP of the node where the VPN tunnel is created
	EndpointIP string `json:"endpointIP"`
	//vpn technology used to interconnect two clusters
	BackendType string `json:"backendType"`
	//connection parameters
	BackendConfig map[string]string `json:"backend_config"`
}

// TunnelEndpointStatus defines the observed state of TunnelEndpoint
type TunnelEndpointStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file\
	Phase                 string     `json:"phase,omitempty"` //two phases: New, Processed
	LocalRemappedPodCIDR  string     `json:"localRemappedPodCIDR,omitempty"`
	LocalPodCIDR          string     `json:"localPodCIDR,omitempty"`
	RemoteRemappedPodCIDR string     `json:"remoteRemappedPodCIDR,omitempty"`
	OutgoingNAT           bool       `json:"outgoingNAT,omitempty"` // if true, the local podCIDR has been remapped by the remote cluster
	IncomingNAT           bool       `json:"incomingNAT,omitempty"` // if true, the remote podCIDR has been remapped by the local cluster
	RemoteEndpointIP      string     `json:"remoteTunnelPublicIP,omitempty"`
	LocalEndpointIP       string     `json:"localTunnelPublicIP,omitempty"`
	TunnelIFaceIndex      int        `json:"tunnelIFaceIndex,omitempty"`
	TunnelIFaceName       string     `json:"tunnelIFaceName,omitempty"`
	Connection            Connection `json:"connection,omitempty"`
}

type Connection struct {
	Status            ConnectionStatus  `json:"status,omitempty"`
	StatusMessage     string            `json:"statusMessage,omitempty"`
	PeerConfiguration map[string]string `json:"peerConfiguration,omitempty"`
}

type ConnectionStatus string

const (
	Connected       ConnectionStatus = "connected"
	Connecting      ConnectionStatus = "connecting"
	ConnectionError ConnectionStatus = "error"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// TunnelEndpoint is the Schema for the endpoints API
type TunnelEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TunnelEndpointSpec   `json:"spec,omitempty"`
	Status TunnelEndpointStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TunnelEndpointList contains a list of TunnelEndpoint
type TunnelEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TunnelEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TunnelEndpoint{}, &TunnelEndpointList{})
}
