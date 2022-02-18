// Copyright 2019-2022 The Liqo Authors
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

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ResourceNetworkConfigs the name of the networmconfigs resources.
var ResourceNetworkConfigs = "networkconfigs"

// NetworkConfigSpec defines the desired state of NetworkConfig.
type NetworkConfigSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The remote cluster that will receive this CRD.
	RemoteCluster discoveryv1alpha1.ClusterIdentity `json:"cluster"`
	// Network used in the local cluster for the pod IPs.
	PodCIDR string `json:"podCIDR"`
	// Network used for local service endpoints.
	ExternalCIDR string `json:"externalCIDR"`
	// Public IP of the node where the VPN tunnel is created.
	EndpointIP string `json:"endpointIP"`
	// Vpn technology used to interconnect two clusters.
	BackendType string `json:"backendType"`
	// Connection parameters
	BackendConfig map[string]string `json:"backend_config"`
}

// NetworkConfigStatus defines the observed state of NetworkConfig.
type NetworkConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Indicates if this network config has been processed by the remote cluster.
	// +kubebuilder:default=false
	Processed bool `json:"processed"`
	// The new subnet used to NAT the podCidr of the remote cluster. The original PodCidr may have been mapped to this
	// network by the remote cluster.
	PodCIDRNAT string `json:"podCIDRNAT,omitempty"`
	// The new subnet used to NAT the externalCIDR of the remote cluster. The original ExternalCIDR may have been mapped
	// to this network by the remote cluster.
	ExternalCIDRNAT string `json:"externalCIDRNAT,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NetworkConfig is the Schema for the networkconfigs API.
// +kubebuilder:printcolumn:name="Peering Cluster ID",type=string,JSONPath=`.spec.clusterID`
// +kubebuilder:printcolumn:name="Endpoint IP",type=string,JSONPath=`.spec.endpointIP`,priority=1
// +kubebuilder:printcolumn:name="VPN Backend",type=string,JSONPath=`.spec.backendType`,priority=1
// +kubebuilder:printcolumn:name="Processed",type=string,JSONPath=`.status.processed`
// +kubebuilder:printcolumn:name="Local",type=string,JSONPath=`.metadata.labels.liqo\.io/replication`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type NetworkConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkConfigSpec   `json:"spec,omitempty"`
	Status NetworkConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NetworkConfigList contains a list of NetworkConfig.
type NetworkConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkConfig{}, &NetworkConfigList{})
}
