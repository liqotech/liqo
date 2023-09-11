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

// GatewayServerResource the name of the gatewayserver resources.
var GatewayServerResource = "gatewayservers"

// GatewayServerKind specifies the kind of the gatewayserver resources.
var GatewayServerKind = "GatewayServer"

// GatewayServerGroupResource specifies the group and the resource of the gatewayserver resources.
var GatewayServerGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: GatewayServerResource}

// GatewayServerGroupVersionResource specifies the group, the version and the resource of the gatewayserver resources.
var GatewayServerGroupVersionResource = GroupVersion.WithResource(GatewayServerResource)

// Endpoint defines the endpoint of the gatewayserver.
type Endpoint struct {
	// Port specifies the port of the endpoint.
	Port int32 `json:"port,omitempty"`
	// ServiceType specifies the type of the service.
	// +kubebuilder:default=ClusterIP
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer;ExternalName
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
}

// GatewayServerSpec defines the desired state of GatewayServer.
type GatewayServerSpec struct {
	// ServerTemplateRef specifies the reference to the server template.
	ServerTemplateRef corev1.ObjectReference `json:"serverTemplateRef,omitempty"`
	// MTU specifies the MTU of the tunnel.
	MTU int `json:"mtu,omitempty"`
	// Endpoint specifies the endpoint of the tunnel.
	Endpoint Endpoint `json:"endpoint,omitempty"`
}

// EndpointStatus defines the observed state of the endpoint.
type EndpointStatus struct {
	// Addresses specifies the addresses of the endpoint.
	Addresses []string `json:"addresses,omitempty"`
	// Port specifies the port of the endpoint.
	Port int32 `json:"port,omitempty"`
	// Protocol specifies the protocol of the endpoint.
	// +kubebuilder:validation:Enum=TCP;UDP
	Protocol *corev1.Protocol `json:"protocol,omitempty"`
}

// GatewayServerStatus defines the observed state of GatewayServer.
type GatewayServerStatus struct {
	// ServerRef specifies the reference to the server.
	ServerRef corev1.ObjectReference `json:"serverRef,omitempty"`
	// Endpoint specifies the endpoint of the tunnel.
	Endpoint *EndpointStatus `json:"endpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status

// GatewayServer defines a gateway server that remote gateway clients need to point to.
type GatewayServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewayServerSpec   `json:"spec,omitempty"`
	Status GatewayServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayServerList contains a list of GatewayServer.
type GatewayServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GatewayServer{}, &GatewayServerList{})
}
