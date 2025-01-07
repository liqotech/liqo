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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WgGatewayClientTemplateResource the name of the wggatewayclienttemplate resources.
var WgGatewayClientTemplateResource = "wggatewayclienttemplates"

// WgGatewayClientTemplateKind is the kind name used to register the WgGatewayClientTemplate CRD.
var WgGatewayClientTemplateKind = "WgGatewayClientTemplate"

// WgGatewayClientTemplateGroupResource is group resource used to register these objects.
var WgGatewayClientTemplateGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: WgGatewayClientTemplateResource}

// WgGatewayClientTemplateGroupVersionResource is groupResourceVersion used to register these objects.
var WgGatewayClientTemplateGroupVersionResource = GroupVersion.WithResource(WgGatewayClientTemplateResource)

// WgGatewayClientTemplateSpec defines the desired state of WgGatewayClientTemplate.
type WgGatewayClientTemplateSpec struct {
	// ObjectKind specifies the kind of the object.
	ObjectKind metav1.TypeMeta `json:"objectKind,omitempty"`
	// Template specifies the template of the client.
	// +kubebuilder:pruning:PreserveUnknownFields
	Template unstructured.Unstructured `json:"template,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=wggct;wgct

// WgGatewayClientTemplate contains a template for a wireguard gateway client.
type WgGatewayClientTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec WgGatewayClientTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// WgGatewayClientTemplateList contains a list of WgGatewayClientTemplate.
type WgGatewayClientTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WgGatewayClientTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WgGatewayClientTemplate{}, &WgGatewayClientTemplateList{})
}
