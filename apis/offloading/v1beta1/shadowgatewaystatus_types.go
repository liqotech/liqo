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

// ShadowGatewayStatusSpec defines the desired state of ShadowGatewayStatus.
type ShadowGatewayStatusSpec struct {
	// GatewayName is the name of the local Gateway whose remote status is reported.
	GatewayName string `json:"gatewayName"`
	// ClusterID is the ID of the remote cluster that owns this status.
	ClusterID string `json:"clusterID"`
	// Addresses are the addresses assigned to the remote Gateway.
	// The maximum number of items matches the one of the Gateway API, and it is required to bound the cost of the validation rules.
	// +optional
	// +kubebuilder:validation:MaxItems=16
	Addresses []gwv1.GatewayStatusAddress `json:"addresses,omitempty"`
	// Conditions are the conditions of the remote Gateway.
	// +optional
	// +kubebuilder:validation:MaxItems=8
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Listeners are the statuses of the listeners of the remote Gateway.
	// +optional
	// +kubebuilder:validation:MaxItems=64
	Listeners []gwv1.ListenerStatus `json:"listeners,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=shgws
// +kubebuilder:printcolumn:name="Gateway",type=string,JSONPath=`.spec.gatewayName`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterID`
// +genclient

// ShadowGatewayStatus reports the status of a Gateway reflected to a remote cluster, which is then aggregated into the local Gateway.
type ShadowGatewayStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ShadowGatewayStatusSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ShadowGatewayStatusList contains a list of ShadowGatewayStatus.
type ShadowGatewayStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShadowGatewayStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ShadowGatewayStatus{}, &ShadowGatewayStatusList{})
}
