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

// PublicKeyResource the name of the publickey resources.
var PublicKeyResource = "publickeies"

// PublicKeyKind is the kind name used to register the PublicKey CRD.
var PublicKeyKind = "PublicKey"

// PublicKeyGroupResource is group resource used to register these objects.
var PublicKeyGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: PublicKeyResource}

// PublicKeyGroupVersionResource is groupResourceVersion used to register these objects.
var PublicKeyGroupVersionResource = GroupVersion.WithResource(PublicKeyResource)

// PublicKeySpec defines the desired state of PublicKey.
type PublicKeySpec struct {
	// PublicKey contains the public key.
	PublicKey []byte `json:"publicKey,omitempty"`
}

// publickeies is used for resource name pluralization because k8s api do not manage false friends.
// Waiting for this fix https://github.com/kubernetes-sigs/kubebuilder/pull/3408

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,path=publickeies,shortName=pk;pkies;pkey

// PublicKey contains a public key data required by some interconnection technologies.
type PublicKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PublicKeySpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// PublicKeyList contains a list of PublicKey.
type PublicKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PublicKey `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PublicKey{}, &PublicKeyList{})
}
