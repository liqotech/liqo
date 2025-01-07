// Copyright 2019-2025 The Liqo Authors
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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// InternalNodeResource the name of the internalNode resources.
var InternalNodeResource = "internalnodes"

// InternalNodeKind is the kind name used to register the InternalNode CRD.
var InternalNodeKind = "InternalNode"

// InternalNodeGroupResource is group resource used to register these objects.
var InternalNodeGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: InternalNodeResource}

// InternalNodeGroupVersionResource is groupResourceVersion used to register these objects.
var InternalNodeGroupVersionResource = GroupVersion.WithResource(InternalNodeResource)

// InternalNodeSpecInterfaceGateway contains the information about the gateway interface.
type InternalNodeSpecInterfaceGateway struct {
	// Name is the name of the interface added to the gateways.
	Name string `json:"name"`
}

// InternalNodeSpecInterfaceNode contains the information about the node interface.
type InternalNodeSpecInterfaceNode struct {
	// IP is the IP of the interface added to the node.
	IP IP `json:"ip"`
}

// InternalNodeSpecInterface contains the information about network interfaces.
type InternalNodeSpecInterface struct {
	// Gateway contains the information about the gateway interface.
	// The gateway interface is created on every gateway to connect them to the node related with the internalnode.
	Gateway InternalNodeSpecInterfaceGateway `json:"gateway"`
	// Node contains the information about the node interface.
	Node InternalNodeSpecInterfaceNode `json:"node"`
}

// InternalNodeSpec defines the desired state of InternalNode.
type InternalNodeSpec struct {
	// Interface contains the information about network interfaces.
	Interface InternalNodeSpecInterface `json:"interface"`
}

// InternalNodeStatusNodeIP defines the observed state of InternalNode.
// It contains the IPs used by an host network pod (scheduled on that node) as src IPs to contact a pod.
type InternalNodeStatusNodeIP struct {
	// Local is the src IP used to contact a pod on the same node.
	Local *IP `json:"local,omitempty"`
	// Remote is the src IP used to contact a pod on another node.
	Remote *IP `json:"remote,omitempty"`
}

// InternalNodeStatus defines the observed state of InternalNode.
type InternalNodeStatus struct {
	// NodeAddress is the address of the node.
	NodeIP InternalNodeStatusNodeIP `json:"nodeIP"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=liqo,shortName=in;inode
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Node IP Local",type=string,JSONPath=`.status.nodeIP.local`
// +kubebuilder:printcolumn:name="Node IP Remote",type=string,JSONPath=`.status.nodeIP.remote`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// InternalNode contains the network internalnode settings.
// Every internalnode resource represents a node in the local cluster.
type InternalNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InternalNodeSpec   `json:"spec,omitempty"`
	Status InternalNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InternalNodeList contains a list of InternalNode.
type InternalNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InternalNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InternalNode{}, &InternalNodeList{})
}
