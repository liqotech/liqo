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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TunnelEndpointSpec defines the desired state of TunnelEndpoint
type TunnelEndpointSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ClusterID       string `json:"clusterID"`
	PodCIDR         string `json:"podCIDR"`
	TunnelPublicIP  string `json:"tunnelPublicIP"`
	TunnelPrivateIP string `json:"tunnelPrivateIP"`
}

// TunnelEndpointStatus defines the observed state of TunnelEndpoint
type TunnelEndpointStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file\
	Phase                 string `json:"phase,omitempty"` //two phases: New, Processed
	LocalRemappedPodCIDR  string `json:"localRemappedPodCIDR,omitempty"`
	RemoteRemappedPodCIDR string `json:"remoteRemappedPodCIDR,omitempty"`
	NATEnabled            bool   `json:"NAT,omitempty"`
	RemoteTunnelPublicIP  string `json:"remoteTunnelPublicIP,omitempty"`
	RemoteTunnelPrivateIP string `json:"remoteTunnelPrivateIP,omitempty"`
	LocalTunnelPublicIP   string `json:"localTunnelPublicIP,omitempty"`
	LocalTunnelPrivateIP  string `json:"localTunnelPrivateIP,omitempty"`
	TunnelIFaceIndex      int    `json:"tunnelIFaceIndex,omitempty"`
	TunnelIFaceName       string `json:"tunnelIFaceName,omitempty"`
}

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
