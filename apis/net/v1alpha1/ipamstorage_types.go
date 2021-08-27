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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ResourceIpamStorages the name of the ipamstorages resources.
var ResourceIpamStorages = "ipamstorages"

// Subnets type contains relevant networks related to a remote cluster.
type Subnets struct {
	// Network used in the remote cluster for local Pods. Default is "None": this means remote cluster uses local cluster PodCIDR.
	LocalNATPodCIDR string `json:"localNATPodCIDR"`
	// Network used in the remote cluster for remote Pods.
	RemotePodCIDR string `json:"remotePodCIDR"`
	// Network used in remote cluster for local service endpoints. Default is "None": this means remote cluster uses local cluster ExternalCIDR.
	LocalNATExternalCIDR string `json:"localNATExternalCIDR"`
	// Network used in remote cluster for remote service endpoints.
	RemoteExternalCIDR string `json:"remoteExternalCIDR"`

	// Network used for Pods in the remote cluster.
	RemoteNATPodCIDR string `json:"remoteNATPodCIDR"`
	// Network used in local cluster for remote service endpoints.
	RemoteNATExternalCIDR string `json:"remoteNATExternalCIDR"`
}

// ClusterMapping is an empty struct.
type ClusterMapping struct{}

// ConfiguredCluster is an empty struct used as value for NatMappingsConfigured.
type ConfiguredCluster struct{}

// EndpointMapping describes a relation between an enpoint IP and an IP belonging to ExternalCIDR.
type EndpointMapping struct {
	// IP belonging to cluster ExtenalCIDR assigned to this endpoint.
	IP string `json:"ip"`
	// Set of clusters to which this endpoint has been reflected. Only the key, which is the ClusterID, is useful.
	ClusterMappings map[string]ClusterMapping `json:"clusterMappings"`
}

// IpamSpec defines the desired state of Ipam.
type IpamSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Map consumed by go-ipam module. Key is prefic cidr, value is a Prefix.
	Prefixes map[string][]byte `json:"prefixes"`
	// Network pools.
	Pools []string `json:"pools"`
	// Reserved Networks. Subnets listed in this field are excluded from the list of possible subnets used for natting POD CIDR.
	ReservedSubnets []string `json:"reservedSubnets"`
	// Map used to keep track of networks assigned to clusters. Key is the remote cluster ID, value is a the set of
	// networks used by the remote cluster.
	ClusterSubnets map[string]Subnets `json:"clusterSubnets"`
	// Cluster ExternalCIDR
	ExternalCIDR string `json:"externalCIDR"`
	// Endpoint IP mappings. Key is the IP address of the local endpoint, value is an EndpointMapping struct
	// that contains the related IP belonging to the ExternalCIDR and also the list of clusters
	// on which this mapping is active
	EndpointMappings map[string]EndpointMapping `json:"endpointMappings"`
	// NatMappingsConfigured is a map that contains all the remote clusters
	// for which NatMappings have been already configured.
	// Key is a cluster ID, value is an empty struct.
	NatMappingsConfigured map[string]ConfiguredCluster `json:"natMappingsConfigured"`
	// Cluster PodCIDR
	PodCIDR string `json:"podCIDR"`
	// ServiceCIDR
	ServiceCIDR string `json:"serviceCIDR"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// IpamStorage is the Schema for the ipams API.
type IpamStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec IpamSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// IpamStorageList contains a list of Ipam.
type IpamStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IpamStorage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IpamStorage{}, &IpamStorageList{})
}
