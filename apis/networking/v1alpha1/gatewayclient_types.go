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

// GatewayClientResource the name of the gatewayclient resources.
var GatewayClientResource = "gatewayclients"

// GatewayClientKind specifies the kind of the gatewayclient resources.
var GatewayClientKind = "GatewayClient"

// GatewayClientGroupResource specifies the group and the resource of the gatewayclient resources.
var GatewayClientGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: GatewayClientResource}

// GatewayClientGroupVersionResource specifies the group, the version and the resource of the gatewayclient resources.
var GatewayClientGroupVersionResource = GroupVersion.WithResource(GatewayClientResource)

// GatewayClientSpec defines the desired state of GatewayClient.
type GatewayClientSpec struct {
	// ClientTemplateRef specifies the reference to the client template.
	ClientTemplateRef corev1.ObjectReference `json:"clientTemplateRef,omitempty"`
	// MTU specifies the MTU of the tunnel.
	MTU int `json:"mtu,omitempty"`
	// Endpoint specifies the endpoint of the tunnel.
	Endpoint Endpoint `json:"endpoint,omitempty"`
}

// GatewayClientStatus defines the observed state of GatewayClient.
type GatewayClientStatus struct {
	// ClientRef specifies the reference to the client.
	ClientRef corev1.ObjectReference `json:"clientRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status

// GatewayClient defines a gateway client that needs to point to a remote gateway server.
type GatewayClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewayClientSpec   `json:"spec,omitempty"`
	Status GatewayClientStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayClientList contains a list of GatewayClient.
type GatewayClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayClient `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GatewayClient{}, &GatewayClientList{})
}
