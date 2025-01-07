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
)

// LimitsEnforcement defines how the quota is enforced.
type LimitsEnforcement string

const (
	// HardLimitsEnforcement means that the quota is enforced with hard limits (limits == requests).
	HardLimitsEnforcement LimitsEnforcement = "Hard"
	// SoftLimitsEnforcement means that the quota is enforced with soft limits (requests <= limits).
	SoftLimitsEnforcement LimitsEnforcement = "Soft"
	// NoLimitsEnforcement means that the quota is not enforced.
	NoLimitsEnforcement LimitsEnforcement = "None"
)

// QuotaSpec defines the desired state of Quota.
type QuotaSpec struct {
	// User is the user for which the quota is defined.
	// +kubebuilder:validation:MinLength=1
	User string `json:"user"`
	// LimitsEnforcement defines how the quota is enforced.
	// +kubebuilder:validation:Enum=Hard;Soft;None
	LimitsEnforcement LimitsEnforcement `json:"limitsEnforcement,omitempty"`
	// Resources contains the list of resources and their limits.
	Resources corev1.ResourceList `json:"resources"`
	// Cordoned indicates if the user is cordoned.
	Cordoned *bool `json:"cordoned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=qt
// +kubebuilder:printcolumn:name="Enforcement",type=string,JSONPath=`.spec.limitsEnforcement`
// +kubebuilder:printcolumn:name="Cordoned",type=boolean,JSONPath=`.spec.cordoned`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="User",type=string,JSONPath=`.spec.user`,priority=1

// Quota is the Schema for the quota API.
type Quota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec QuotaSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// QuotaList contains a list of Quota.
type QuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Quota `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Quota{}, &QuotaList{})
}
