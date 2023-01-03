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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OfferStateType defines the state of the child ResourceOffer resource.
type OfferStateType string

const (
	// OfferStateCreated indicates that the child ResourceOffer resource has been created.
	OfferStateCreated OfferStateType = "Created"
	// OfferStateNone indicates that the child ResourceOffer resource has not been created.
	OfferStateNone OfferStateType = "None"
)

// ResourceRequestSpec defines the desired state of ResourceRequest.
type ResourceRequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foreign Cluster Identity
	ClusterIdentity ClusterIdentity `json:"clusterIdentity"`
	// Local auth service address
	AuthURL string `json:"authUrl"`
	// WithdrawalTimestamp is set when a graceful deletion is requested by the user.
	WithdrawalTimestamp *metav1.Time `json:"withdrawalTimestamp,omitempty"`
}

// ResourceRequestStatus defines the observed state of ResourceRequest.
type ResourceRequestStatus struct {
	// OfferWithdrawalTimestamp is the withdrawal timestamp of the child ResourceOffer resource.
	OfferWithdrawalTimestamp *metav1.Time `json:"offerWithdrawalTimestamp,omitempty"`
	// +kubebuilder:validation:Enum="None";"Created"
	// +kubebuilder:default="None"
	OfferState OfferStateType `json:"offerState"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo
// +kubebuilder:subresource:status

// ResourceRequest is the Schema for the ResourceRequests API.
// +kubebuilder:printcolumn:name="Local",type=string,JSONPath=`.metadata.labels.liqo\.io/replication`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ResourceRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceRequestSpec   `json:"spec,omitempty"`
	Status ResourceRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceRequestList contains a list of ResourceRequest.
type ResourceRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceRequest{}, &ResourceRequestList{})
}
