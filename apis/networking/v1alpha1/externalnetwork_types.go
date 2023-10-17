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
	"k8s.io/apimachinery/pkg/runtime/schema"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ExternalNetworkResource the name of the external network resources.
var ExternalNetworkResource = "externalnetworks"

// ExternalNetworkKind is the kind name used to register the ExternalNetwork CRD.
var ExternalNetworkKind = "ExternalNetwork"

// ExternalNetworkGroupResource is group resource used to register these objects.
var ExternalNetworkGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: ExternalNetworkResource}

// ExternalNetworkGroupVersionResource is groupResourceVersion used to register these objects.
var ExternalNetworkGroupVersionResource = GroupVersion.WithResource(ExternalNetworkResource)

// ExternalNetworkSpec defines the desired state of ExternalNetworks.
type ExternalNetworkSpec struct {
	// Configuration contains the configuration for the originating cluster.
	Configuration *ConfigurationSpec `json:"configuration,omitempty"`
	// ServerEndpoint contains the endpoint of the originating cluster.
	ServerEndpoint *EndpointStatus `json:"serverEndpoint,omitempty"`
	// PublicKey contains the public key of the originating cluster.
	PublicKey []byte `json:"publicKey,omitempty"`
	// ClusterIdentity contains the identity of the originating cluster.
	ClusterIdentity *discoveryv1alpha1.ClusterIdentity `json:"clusterIdentity,omitempty"`
}

// ExternalNetworkStatus defines the observed state of ExternalNetworks.
type ExternalNetworkStatus struct {
	// Configuration contains the configuration for the target cluster.
	Configuration *ConfigurationSpec `json:"configuration,omitempty"`
	// PublicKey contains the public key of the target cluster.
	PublicKey []byte `json:"publicKey,omitempty"`
	// ClusterIdentity contains the identity of the target cluster.
	ClusterIdentity *discoveryv1alpha1.ClusterIdentity `json:"clusterIdentity,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status

// ExternalNetwork is used to share information about the network configuration of a
// peer of clusters. It is replicated by the CRD replicator.
type ExternalNetwork struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalNetworkSpec   `json:"spec,omitempty"`
	Status ExternalNetworkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExternalNetworkList contains a list of ExternalNetworks.
type ExternalNetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExternalNetwork `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalNetwork{}, &ExternalNetworkList{})
}
