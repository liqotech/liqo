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

// RenewResource is the name of the renew resources.
var RenewResource = "renews"

// RenewKind specifies the kind of the renew.
var RenewKind = "Renew"

// RenewGroupResource is group resource used to register these objects.
var RenewGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: RenewResource}

// RenewGroupVersionResource is groupResourceVersion used to register these objects.
var RenewGroupVersionResource = GroupVersion.WithResource(RenewResource)

// RenewSpec defines the desired state of Renew.
type RenewSpec struct {
	// ConsumerClusterID is the id of the consumer cluster.
	ConsumerClusterID liqov1beta1.ClusterID `json:"consumerClusterID,omitempty"`
	// PublicKey is the public key of the tenant cluster.
	PublicKey []byte `json:"publicKey,omitempty"`
	// CSR is the Certificate Signing Request of the tenant cluster.
	CSR []byte `json:"csr,omitempty"`
	// IdentityType is the type of the identity.
	IdentityType IdentityType `json:"identityType,omitempty"`
	// ResoruceSliceRef is the reference to the resource slice.
	ResourceSliceRef *corev1.LocalObjectReference `json:"resourceSliceRef,omitempty"`
}

// RenewStatus defines the observed state of Renew.
type RenewStatus struct {
	// AuthParams contains the authentication parameters for the consumer cluster.
	AuthParams *AuthParams `json:"authParams,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Renew represents a slice of resources given by the provider cluster to the consumer cluster.
type Renew struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RenewSpec   `json:"spec,omitempty"`
	Status RenewStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RenewList contains a list of Renews.
type RenewList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Renew `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Renew{}, &RenewList{})
}
