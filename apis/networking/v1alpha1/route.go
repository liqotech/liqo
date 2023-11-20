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

// RouteResource the name of the route resources.
var RouteResource = "routes"

// RouteKind is the kind name used to register the Route CRD.
var RouteKind = "Route"

// RouteGroupResource is group resource used to register these objects.
var RouteGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: RouteResource}

// RouteGroupVersionResource is groupResourceVersion used to register these objects.
var RouteGroupVersionResource = GroupVersion.WithResource(RouteResource)

// RouteDestination defines the destination of the route.
type RouteDestination struct {
	// IP is the IP address of the destination. It is mutually exclusive with CIDR.
	IP *string `json:"ip,omitempty"`
	// CIDR is the CIDR of the destination. It is mutually exclusive with IP.
	CIDR *string `json:"cidr,omitempty"`
}

// RouteNextHop defines the next hop of the route.
type RouteNextHop struct {
	// IP is the IP address of the next hop. It is mutually exclusive with Dev.
	IP *string `json:"ip,omitempty"`
	// Dev is the name of the device of the next hop. It is mutually exclusive with IP.
	Dev *string `json:"dev,omitempty"`
}

// RouteSpec defines the desired state of Route.
type RouteSpec struct {
	// Dest is the destination of the route.
	Dest *RouteDestination `json:"dest,omitempty"`
	// NextHop is the next hop of the route.
	NextHop *RouteNextHop `json:"nextHop,omitempty"`
	// Table is the table of the route.
	Table string `json:"table,omitempty"`
}

// RouteConditionType is a valid value for RouteCondition.Type.
type RouteConditionType string

const (
	// RouteConditionTypeApplied means the route has been applied.
	RouteConditionTypeApplied RouteConditionType = "Applied"
)

// RouteCondition contains details for the current condition of this route.
type RouteCondition struct {
	// Type is the type of the condition.
	// +kubebuilder:validation:Enum=Applied
	Type RouteConditionType `json:"type"`
	// Status is the status of the condition.
	// +kubebuilder:validation:Enum=True;False;Unknown
	// +kubebuilder:default=Unknown
	Status corev1.ConditionStatus `json:"status"`
	// LastTransitionTime is the last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message is a human-readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

// RouteStatus defines the observed state of Route.
type RouteStatus struct {
	// Conditions contains information about the current status of the route.
	Conditions []RouteCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Destination IP",type=string,JSONPath=`.spec.dest.ip`
// +kubebuilder:printcolumn:name="Destination CIDR",type=string,JSONPath=`.spec.dest.cidr`
// +kubebuilder:printcolumn:name="Next Hop IP",type=string,JSONPath=`.spec.nextHop.ip`, priority=1
// +kubebuilder:printcolumn:name="Next Hop Dev",type=string,JSONPath=`.spec.nextHop.dev`, priority=1
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.conditions[?(@.type == 'Applied')].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Route contains the network route of a pair of clusters,
// including the local and the remote pod and external CIDRs and how the where remapped.
type Route struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteSpec   `json:"spec,omitempty"`
	Status RouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RouteList contains a list of Route.
type RouteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Route `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Route{}, &RouteList{})
}
