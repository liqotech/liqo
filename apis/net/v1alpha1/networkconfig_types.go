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
	"github.com/liqotech/liqo/pkg/crdClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NetworkConfigSpec defines the desired state of NetworkConfig
type NetworkConfigSpec struct {
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

// NetworkConfigStatus defines the observed state of NetworkConfig
type NetworkConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	//indicates if the NAT is enabled for the remote cluster
	NATEnabled string `json:"natEnabled,omitempty"`
	//the new subnet used to NAT the pods' subnet of the remote cluster
	PodCIDRNAT string `json:"podCIDRNAT,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// NetworkConfig is the Schema for the networkconfigs API
type NetworkConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkConfigSpec   `json:"spec,omitempty"`
	Status NetworkConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkConfigList contains a list of NetworkConfig
type NetworkConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkConfig{}, &NetworkConfigList{})

	crdClient.AddToRegistry("networkconfigs", &NetworkConfig{}, &NetworkConfigList{}, nil, schema.GroupResource{
		Group:    TunnelEndpointGroupResource.Group,
		Resource: "networkconfigs",
	})
}
