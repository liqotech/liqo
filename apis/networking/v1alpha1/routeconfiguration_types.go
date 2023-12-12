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

// RouteConfigurationResource the name of the routeconfiguration resources.
var RouteConfigurationResource = "routeconfigurations"

// RouteConfigurationKind is the kind name used to register the RouteConfiguration CRD.
var RouteConfigurationKind = "RouteConfiguration"

// RouteConfigurationGroupResource is group resource used to register these objects.
var RouteConfigurationGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: RouteConfigurationResource}

// RouteConfigurationGroupVersionResource is groupResourceVersion used to register these objects.
var RouteConfigurationGroupVersionResource = GroupVersion.WithResource(RouteConfigurationResource)

// Route is the route of the RouteConfiguration.
type Route struct {
	// Dst is the destination of the RouteConfiguration.
	Dst *CIDR `json:"dst"`
	// Src is the source of the RouteConfiguration.
	Src *IP `json:"src,omitempty"`
	// Gw is the gateway of the RouteConfiguration.
	Gw *IP `json:"gw,omitempty"`
	// Dev is the device of the RouteConfiguration.
	Dev *string `json:"dev,omitempty"`
}

// Rule is the rule of the RouteConfiguration.
type Rule struct {
	// Dst is the destination of the Rule.
	Dst *CIDR `json:"dst,omitempty"`
	// Src is the source of the Rule.
	Src *CIDR `json:"src,omitempty"`
	// Iif is the input interface name of the Rule.
	Iif *string `json:"iif,omitempty"`
	// OifName is the output interface name of the Rule.
	Oif *string `json:"oif,omitempty"`
	// Routes is the list of routes of the Rule.
	// +kubebuilder:validation:MinItems=1
	Routes []Route `json:"routes"`
}

// Table is the table of the RouteConfiguration.
type Table struct {
	// Name is the name of the table of the RouteConfiguration.
	Name string `json:"name"`
	// Rules is the list of rules of the RouteConfiguration.
	// +kubebuilder:validation:MinItems=1
	Rules []Rule `json:"rules"`
}

// RouteConfigurationSpec defines the desired state of RouteConfiguration.
type RouteConfigurationSpec struct {
	// Table is the table of the RouteConfiguration.
	Table Table `json:"table,omitempty"`
}

// RouteConfigurationStatusCondition defines the observed state of FirewallConfiguration.
type RouteConfigurationStatusCondition string

const (
	// RouteConfigurationStatusConditionApplied reports that the configuration has been applied.
	RouteConfigurationStatusConditionApplied RouteConfigurationStatusCondition = "Applied"
	// RouteConfigurationStatusConditionError reports an error in the configuration.
	RouteConfigurationStatusConditionError RouteConfigurationStatusCondition = "Error"
)

// RouteConfigurationStatus defines the observed state of RouteConfiguration.
type RouteConfigurationStatus struct {
	// Condition is the condition of the RouteConfiguration
	Condition RouteConfigurationStatusCondition `json:"condition,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.condition`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RouteConfiguration contains the network RouteConfiguration of a pair of clusters,
// including the local and the remote pod and external CIDRs and how the where remapped.
type RouteConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RouteConfigurationSpec   `json:"spec,omitempty"`
	Status RouteConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RouteConfigurationList contains a list of RouteConfiguration.
type RouteConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RouteConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RouteConfiguration{}, &RouteConfigurationList{})
}
