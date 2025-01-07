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

// Scope is the scope of the route.
type Scope string

const (
	// GlobalScope is the global scope of the RouteConfiguration.
	GlobalScope Scope = "global"
	// LinkScope is the link scope of the RouteConfiguration.
	LinkScope Scope = "link"
	// HostScope is the host scope of the RouteConfiguration.
	HostScope Scope = "host"
	// SiteScope is the site scope of the RouteConfiguration.
	SiteScope Scope = "site"
	// NowhereScope is the nowhere scope of the RouteConfiguration.
	NowhereScope Scope = "nowhere"
)

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
	// Onlink enables the onlink falg inside the route.
	Onlink *bool `json:"onlink,omitempty"`
	// Scope is the scope of the RouteConfiguration.
	// +kubebuilder:validation:Enum=global;link;host;site;nowhere
	Scope *Scope `json:"scope,omitempty"`
	// TargetRef is the reference to the target object of the route.
	// It is optional and it can be used for custom purposes.
	TargetRef *corev1.ObjectReference `json:"targetRef,omitempty"`
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
	// FwMark is the firewall mark of the Rule.
	FwMark *int `json:"fwmark,omitempty"`
	// Routes is the list of routes of the Rule.
	Routes []Route `json:"routes"`
	// TargetRef is the reference to the target object of the rule.
	// It is optional and it can be used for custom purposes.
	TargetRef *corev1.ObjectReference `json:"targetRef,omitempty"`
}

// Table is the table of the RouteConfiguration.
type Table struct {
	// Name is the name of the table of the RouteConfiguration.
	Name string `json:"name"`
	// Rules is the list of rules of the RouteConfiguration.
	// +kubebuilder:validation:MinItems=1
	Rules []Rule `json:"rules"`
	// TargetRef is the reference to the target object of the table.
	// It is optional and it can be used for custom purposes.
	TargetRef *corev1.ObjectReference `json:"targetRef,omitempty"`
}

// RouteConfigurationSpec defines the desired state of RouteConfiguration.
type RouteConfigurationSpec struct {
	// Table is the table of the RouteConfiguration.
	Table Table `json:"table,omitempty"`
}

// RouteConfigurationStatusConditionType is a type of routeconfiguration condition.
type RouteConfigurationStatusConditionType string

const (
	// RouteConfigurationStatusConditionTypeApplied reports that the configuration has been applied.
	RouteConfigurationStatusConditionTypeApplied RouteConfigurationStatusConditionType = "Applied"
	// RouteConfigurationStatusConditionTypeError reports an error in the configuration.
	RouteConfigurationStatusConditionTypeError RouteConfigurationStatusConditionType = "Error"
)

// RouteConfigurationStatusCondition defines the observed state of FirewallConfiguration.
type RouteConfigurationStatusCondition struct {
	// Host where the configuration has been applied.
	Host string `json:"host"`
	// Type of routeconfiguration condition.
	Type RouteConfigurationStatusConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status metav1.ConditionStatus `json:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// RouteConfigurationStatus defines the observed state of RouteConfiguration.
type RouteConfigurationStatus struct {
	// Conditions is the list of conditions of the RouteConfiguration.
	Conditions []RouteConfigurationStatusCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=rcnf;rcnfg;rcfg;routecfg
// +kubebuilder:subresource:status
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
