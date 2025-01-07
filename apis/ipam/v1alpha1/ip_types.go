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

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

var (
	// IPKind is the kind name used to register the IP CRD.
	IPKind = "IP"

	// IPResource is the resource name used to register the IP CRD.
	IPResource = "ips"

	// IPGroupVersionResource is the group version resource used to register IP CRD.
	IPGroupVersionResource = SchemeGroupVersion.WithResource(IPResource)

	// IPGroupResource is the group resource used to register IP CRD.
	IPGroupResource = schema.GroupResource{Group: SchemeGroupVersion.Group, Resource: IPResource}
)

// ServiceTemplate contains the template to create the associated service (and endpointslice) for the IP endopoint.
type ServiceTemplate struct {
	// Metadata of the Service.
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	// Template Spec of the Service.
	Spec v1.ServiceSpec `json:"spec,omitempty"`
}

// IPSpec defines a local IP.
type IPSpec struct {
	// IP is the local IP.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="IP field is immutable"
	IP networkingv1beta1.IP `json:"ip"`
	// NetworkRef is the reference to the Network CR containing the CIDR where the desired IP should be allocated from.
	// It is optional, if left empty the IP will be allocated in a default network CIDR (e.g., external CIDR).
	// +kubebuilder:validation:Optional
	NetworkRef *v1.ObjectReference `json:"networkRef,omitempty"`
	// ServiceTemplate contains the template to create the associated service (and endpointslice) for the IP endopoint.
	// If empty the creation of the service is disabled (default).
	// +kubebuilder:validation:Optional
	ServiceTemplate *ServiceTemplate `json:"serviceTemplate,omitempty"`
	// Masquerade is a flag to enable masquerade for the local IP on nodes.
	// If empty the masquerade is disabled.
	// +kubebuilder:validation:Optional
	Masquerade *bool `json:"masquerade,omitempty"`
}

// IPStatus defines remapped IPs.
type IPStatus struct {
	// IP is the remapped IP.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="IP field is immutable"
	IP networkingv1beta1.IP `json:"ip,omitempty"`
	// CIDR is the network CIDR where the IP is allocated.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="CIDR field is immutable"
	CIDR networkingv1beta1.CIDR `json:"cidr,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Local IP",type=string,JSONPath=`.spec.ip`
// +kubebuilder:printcolumn:name="Remapped IP",type=string,JSONPath=`.status.ip`
// +kubebuilder:printcolumn:name="Remapped IP CIDR",type=string,JSONPath=`.status.cidr`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +genclient

// IP is the Schema for the IP API.
type IP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IPSpec   `json:"spec"`
	Status IPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IPList contains a list of IP.
type IPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IP{}, &IPList{})
}
