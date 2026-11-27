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
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ShadowIngressStatusSpec defines the desired state of ShadowIngressStatus.
type ShadowIngressStatusSpec struct {
	// IngressName is the name of the local Ingress whose remote status is reported.
	IngressName string `json:"ingressName"`
	// ClusterID is the ID of the remote cluster that owns this status.
	ClusterID string `json:"clusterID"`
	// LoadBalancer is the load-balancer status of the remote Ingress.
	LoadBalancer netv1.IngressLoadBalancerStatus `json:"loadBalancer,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=shis
// +genclient

// ShadowIngressStatus is the Schema for the ShadowIngressStatus API.
type ShadowIngressStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ShadowIngressStatusSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ShadowIngressStatusList contains a list of ShadowIngressStatus.
type ShadowIngressStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ShadowIngressStatus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ShadowIngressStatus{}, &ShadowIngressStatusList{})
}
