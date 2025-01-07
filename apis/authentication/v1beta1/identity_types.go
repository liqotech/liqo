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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

// IdentityResource is the name of the identity resources.
var IdentityResource = "identities"

// IdentityKind specifies the kind of the identity.
var IdentityKind = "Identity"

// IdentityGroupResource is group resource used to register these objects.
var IdentityGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: IdentityResource}

// IdentityGroupVersionResource is groupResourceVersion used to register these objects.
var IdentityGroupVersionResource = GroupVersion.WithResource(IdentityResource)

// IdentityType is the type of the Identity. It can be either "ControlPlane" or "VirtualNode":
//   - ControlPlane: identity that gives the permissions to replicate resources to the remote
//     cluster through the CrdReplicator.
//   - VirtualNode: identity that gives the permissions to create a virtual node.
type IdentityType string

const (
	// ControlPlaneIdentityType indicates an Identity of type ControlPlane.
	ControlPlaneIdentityType IdentityType = "ControlPlane"
	// ResourceSliceIdentityType indicates an Identity of type ResourceSlice.
	ResourceSliceIdentityType IdentityType = "ResourceSlice"
)

// IdentitySpec defines the desired state of Identity.
type IdentitySpec struct {
	// ClusterID is the identity of the provider cluster.
	ClusterID liqov1beta1.ClusterID `json:"clusterID,omitempty"`
	// Type is the type of the identity.
	// +kubebuilder:validation:Enum=ControlPlane;ResourceSlice
	Type IdentityType `json:"type,omitempty"`
	// AuthParams contains the parameters to create an Identity to use in the provider cluster.
	AuthParams AuthParams `json:"authParams,omitempty"`
	// Namespace is the namespace where to use the identity.
	// +kubebuilder:validation:Optional
	Namespace *string `json:"namespace,omitempty"`
}

// IdentityStatus defines the observed state of Identity.
type IdentityStatus struct {
	// KubeconfigSecretRef contains the reference to the secret containing the kubeconfig to access the provider cluster.
	KubeconfigSecretRef *corev1.LocalObjectReference `json:"kubeconfigSecretRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=id
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="KubeconfigSecret",type=string,JSONPath=`.status.kubeconfigSecretRef.name`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Identity contains the information to operate in a remote cluster.
type Identity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IdentitySpec   `json:"spec,omitempty"`
	Status IdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IdentityList contains a list of Identities.
type IdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Identity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Identity{}, &IdentityList{})
}
