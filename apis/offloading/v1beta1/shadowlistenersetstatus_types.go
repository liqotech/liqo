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

// ShadowListenerSetStatusSpec defines the desired state of ShadowListenerSetStatus.
type ShadowListenerSetStatusSpec struct {
	// ListenerSetName is the name of the local ListenerSet whose remote status is reported.
	ListenerSetName string `json:"listenerSetName"`
	// ClusterID is the ID of the remote cluster that owns this status.
	ClusterID string `json:"clusterID"`
	// Conditions are the conditions of the remote ListenerSet.
	// The maximum number of items matches the one of the Gateway API, and it is required to bound the cost of the validation rules.
	// +optional
	// +kubebuilder:validation:MaxItems=8
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Listeners are the statuses of the listeners of the remote ListenerSet.
	// +optional
	// +kubebuilder:validation:MaxItems=64
	Listeners []gwv1.ListenerEntryStatus `json:"listeners,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=shls
// +kubebuilder:printcolumn:name="ListenerSet",type=string,JSONPath=`.spec.listenerSetName`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterID`
// +genclient

// ShadowListenerSetStatus reports the status of a ListenerSet reflected to a remote cluster, which is then aggregated into the local ListenerSet.
type ShadowListenerSetStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ShadowListenerSetStatusSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ShadowListenerSetStatusList contains a list of ShadowListenerSetStatus.
type ShadowListenerSetStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShadowListenerSetStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ShadowListenerSetStatus{}, &ShadowListenerSetStatusList{})
}
