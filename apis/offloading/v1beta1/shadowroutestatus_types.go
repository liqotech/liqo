// Copyright 2019-2026 The Liqo Authors
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
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// RouteKind is the kind of a Gateway API route whose status is reported.
// +kubebuilder:validation:Enum=HTTPRoute;GRPCRoute
type RouteKind string

const (
	// HTTPRouteKind is the kind of the HTTPRoute resource.
	HTTPRouteKind RouteKind = "HTTPRoute"
	// GRPCRouteKind is the kind of the GRPCRoute resource.
	GRPCRouteKind RouteKind = "GRPCRoute"
)

// ShadowRouteStatusSpec defines the desired state of ShadowRouteStatus.
type ShadowRouteStatusSpec struct {
	// Kind is the kind of the local route whose remote status is reported.
	Kind RouteKind `json:"kind"`
	// RouteName is the name of the local route whose remote status is reported.
	RouteName string `json:"routeName"`
	// ClusterID is the ID of the remote cluster that owns this status.
	ClusterID string `json:"clusterID"`
	// Parents are the statuses of the remote route with respect to its parents,
	// whose references are translated to the ones of the local route.
	// The maximum number of items matches the one of the Gateway API, and it is required to bound the cost of the validation rules.
	// +optional
	// +kubebuilder:validation:MaxItems=32
	Parents []gwv1.RouteParentStatus `json:"parents,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=shrs
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.spec.kind`
// +kubebuilder:printcolumn:name="Route",type=string,JSONPath=`.spec.routeName`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterID`
// +genclient

// ShadowRouteStatus reports the status of a route reflected to a remote cluster, which is then aggregated into the local route.
type ShadowRouteStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ShadowRouteStatusSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ShadowRouteStatusList contains a list of ShadowRouteStatus.
type ShadowRouteStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShadowRouteStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ShadowRouteStatus{}, &ShadowRouteStatusList{})
}
