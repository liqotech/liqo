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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// InternalFabricResource the name of the internalFabric resources.
var InternalFabricResource = "internalfabrics"

// InternalFabricKind is the kind name used to register the InternalFabric CRD.
var InternalFabricKind = "InternalFabric"

// InternalFabricGroupResource is group resource used to register these objects.
var InternalFabricGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: InternalFabricResource}

// InternalFabricGroupVersionResource is groupResourceVersion used to register these objects.
var InternalFabricGroupVersionResource = GroupVersion.WithResource(InternalFabricResource)

// InternalEndpoint defines the endpoint of the internal fabric.
type InternalEndpoint struct {
	// IP is the IP address of the endpoint.
	IP IP `json:"ip,omitempty"`
	// Port is the port of the endpoint.
	Port int32 `json:"port,omitempty"`
}

// InternalFabricSpecInterfaceNode contains the information about the node interface.
type InternalFabricSpecInterfaceNode struct {
	// Name is the name of the node interface.
	Name string `json:"name"`
}

// InternalFabricSpecInterface contains the information about network interfaces.
type InternalFabricSpecInterface struct {
	// Node contains the information about the node interface.
	Node InternalFabricSpecInterfaceNode `json:"node"`
}

// InternalFabricSpec defines the desired state of InternalFabric.
type InternalFabricSpec struct {
	// MTU is the MTU of the internal fabric.
	MTU int `json:"mtu,omitempty"`
	// RemoteCIDRs is the list of remote CIDRs to be routed through the gateway.
	RemoteCIDRs []CIDR `json:"remoteCIDRs,omitempty"`
	// Interface contains the information about network interfaces.
	Interface InternalNodeSpecInterface `json:"interface"`
	//GatewayAddress is the address of the gateway pod
	GatewayAddress string `json:"gatewayAddress"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:printcolumn:name="Gateway Node",type=string,JSONPath=`.spec.nodeName`
// +kubebuilder:printcolumn:name="Gateway IP",type=string,JSONPath=`.spec.endpoint.ip`
// +kubebuilder:printcolumn:name="Gateway Port",type=string,JSONPath=`.spec.endpoint.port`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// InternalFabric contains the network internalfabric of a pair of clusters,
// including the local and the remote pod and external CIDRs and how the where remapped.
type InternalFabric struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InternalFabricSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// InternalFabricList contains a list of InternalFabric.
type InternalFabricList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InternalFabric `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InternalFabric{}, &InternalFabricList{})
}
