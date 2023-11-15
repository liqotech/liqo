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

	firewallapi "github.com/liqotech/liqo/apis/networking/v1alpha1/firewall"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FirewallConfigurationSpec defines the desired state of FirewallConfiguration.
type FirewallConfigurationSpec struct {
	// Table contains the rules to be applied to the firewall.
	Table firewallapi.Table `json:"table"`
}

// FirewallConfigurationStatusCondition defines the observed state of FirewallConfiguration.
type FirewallConfigurationStatusCondition string

const (
	// FirewallConfigurationStatusConditionApplied is true if the configuration has been applied to the firewall.
	FirewallConfigurationStatusConditionApplied FirewallConfigurationStatusCondition = "Applied"
	// FirewallConfigurationStatusConditionError is true if the configuration has not been applied to the firewall.
	FirewallConfigurationStatusConditionError FirewallConfigurationStatusCondition = "Error"
)

// FirewallConfigurationStatus defines the observed state of FirewallConfiguration.
type FirewallConfigurationStatus struct {
	// Applied is true if the configuration has been applied to the firewall.
	Condition FirewallConfigurationStatusCondition `json:"condition,omitempty"`
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
