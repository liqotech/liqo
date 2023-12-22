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

// GeneveTunnelResource the name of the geneveTunnel resources.
var GeneveTunnelResource = "genevetunnels"

// GeneveTunnelKind is the kind name used to register the GeneveTunnel CRD.
var GeneveTunnelKind = "GeneveTunnel"

// GeneveTunnelGroupResource is group resource used to register these objects.
var GeneveTunnelGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: GeneveTunnelResource}

// GeneveTunnelGroupVersionResource is groupResourceVersion used to register these objects.
var GeneveTunnelGroupVersionResource = GroupVersion.WithResource(GeneveTunnelResource)

// GeneveTunnelSpec defines the desired state of GeneveTunnel.
type GeneveTunnelSpec struct {
	// The ID of the geneve tunnel. Only 24 can be used.
	ID uint32 `json:"id"`
	// InternalNodeRef is the reference to the internal node.
	InternalNodeRef *corev1.ObjectReference `json:"internalNodeRef"`
	// InternalFabricRef is the reference to the internal fabric.
	InternalFabricRef *corev1.ObjectReference `json:"internalFabricRef"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo

// GeneveTunnel contains the settings about a geneve tunnel.
// It links an InternalNode to an InternalFabric.
type GeneveTunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GeneveTunnelSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// GeneveTunnelList contains a list of GeneveTunnel.
type GeneveTunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GeneveTunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GeneveTunnel{}, &GeneveTunnelList{})
}
