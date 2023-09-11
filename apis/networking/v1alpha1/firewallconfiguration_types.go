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

// FirewallConfigurationResource the name of the firewallconfiguration resources.
var FirewallConfigurationResource = "firewallconfigurations"

// FirewallConfigurationKind is the kind name used to register the FirewallConfiguration CRD.
var FirewallConfigurationKind = "FirewallConfiguration"

// FirewallConfigurationGroupResource is group resource used to register these objects.
var FirewallConfigurationGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: FirewallConfigurationResource}

// FirewallConfigurationGroupVersionResource is groupResourceVersion used to register these objects.
var FirewallConfigurationGroupVersionResource = GroupVersion.WithResource(FirewallConfigurationResource)

// AddRemove contains the commands to add or remove rules.
type AddRemove struct {
	// Add contains the commands to add rules.
	Add []string `json:"add,omitempty"`
	// Remove contains the commands to remove rules.
	Remove []string `json:"remove,omitempty"`
}

// FirewallConfigurationSpec defines the desired state of FirewallConfiguration.
type FirewallConfigurationSpec struct {
	// Command to add or remove rules.
	Command AddRemove `json:"command,omitempty"`
	// ExpectedRule contains the expected rule.
	ExpectedRule string `json:"expectedRule,omitempty"`
	// Table contains the table where the rule is applied.
	Table string `json:"table,omitempty"`
}

// FirewallConfigurationConditionType represents different conditions that a firewallconfiguration could assume.
type FirewallConfigurationConditionType string

const (
	// FirewallConfigurationConditionApplied represents the condition applied.
	FirewallConfigurationConditionApplied FirewallConfigurationConditionType = "Applied"
	// FirewallConfigurationConditionError represents the condition error.
	FirewallConfigurationConditionError FirewallConfigurationConditionType = "Error"
	// FirewallConfigurationConditionPending represents the condition pending.
	FirewallConfigurationConditionPending FirewallConfigurationConditionType = "Pending"
)

// FirewallConfigurationConditionStatusType represents the status of a firewallconfiguration condition.
type FirewallConfigurationConditionStatusType string

const (
	// FirewallConfigurationConditionStatusTrue represents the condition status true.
	FirewallConfigurationConditionStatusTrue FirewallConfigurationConditionStatusType = "True"
	// FirewallConfigurationConditionStatusFalse represents the condition status false.
	FirewallConfigurationConditionStatusFalse FirewallConfigurationConditionStatusType = "False"
	// FirewallConfigurationConditionStatusUnknown represents the condition status unknown.
	FirewallConfigurationConditionStatusUnknown FirewallConfigurationConditionStatusType = "Unknown"
)

// FirewallConfigurationCondition contains details about state of the firewallconfiguration.
type FirewallConfigurationCondition struct {
	// Type of the firewallconfiguration condition.
	// +kubebuilder:validation:Enum="Applied"
	Type FirewallConfigurationConditionType `json:"type"`
	// Status of the condition.
	// +kubebuilder:validation:Enum="True";"False";"Unknown"
	// +kubebuilder:default="Unknown"
	Status FirewallConfigurationConditionStatusType `json:"status"`
	// LastTransitionTime -> timestamp for when the condition last transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason -> Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message -> Human-readable message indicating details about the last status transition.
	Message string `json:"message,omitempty"`
}

// FirewallConfigurationStatus defines the observed state of FirewallConfiguration.
type FirewallConfigurationStatus struct {
	// Conditions contains the conditions of the firewallconfiguration.
	Conditions []FirewallConfigurationCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status

// FirewallConfiguration contains a rule to be applied to the firewall in the gateway.
type FirewallConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FirewallConfigurationSpec   `json:"spec,omitempty"`
	Status FirewallConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FirewallConfigurationList contains a list of FirewallConfiguration.
type FirewallConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FirewallConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FirewallConfiguration{}, &FirewallConfigurationList{})
}
