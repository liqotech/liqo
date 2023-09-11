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

// WgGatewayClientResource the name of the wggatewayclient resources.
var WgGatewayClientResource = "wggatewayclients"

// WgGatewayClientKind is the kind name used to register the WgGatewayClient CRD.
var WgGatewayClientKind = "WgGatewayClient"

// WgGatewayClientGroupResource is group resource used to register these objects.
var WgGatewayClientGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: WgGatewayClientResource}

// WgGatewayClientGroupVersionResource is groupResourceVersion used to register these objects.
var WgGatewayClientGroupVersionResource = GroupVersion.WithResource(WgGatewayClientResource)

// WgGatewayClientSpec defines the desired state of WgGatewayClient.
type WgGatewayClientSpec struct {
	// MTU specifies the MTU of the tunnel.
	MTU int `json:"mtu"`
	// Deployment specifies the deployment template for the client.
	Deployment DeploymentTemplate `json:"deployment"`
}

// WgGatewayClientStatus defines the observed state of WgGatewayClient.
type WgGatewayClientStatus struct {
	// SecretRef specifies the reference to the secret.
	SecretRef corev1.ObjectReference `json:"secretRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status

// WgGatewayClient defines a wireguard gateway client that needs to point to a remote wireguard gateway server.
type WgGatewayClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WgGatewayClientSpec   `json:"spec,omitempty"`
	Status WgGatewayClientStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WgGatewayClientList contains a list of WgGatewayClient.
type WgGatewayClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WgGatewayClient `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WgGatewayClient{}, &WgGatewayClientList{})
}
