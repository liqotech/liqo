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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceResourceOffer the name of the resourceoffers resources.
var ResourceResourceOffer = "resourceoffers"

// StorageType defines the type of storage offered by a resource offer.
type StorageType struct {
	// StorageClassName indicates the name of the storage class.
	StorageClassName string `json:"storageClassName"`
	// Default indicates whether this storage class is the default storage class for Liqo.
	Default bool `json:"default,omitempty"`
}

// ResourceOfferSpec defines the desired state of ResourceOffer.
type ResourceOfferSpec struct {
	// ClusterID is the identifier of the cluster that is sending this ResourceOffer.
	// It is the uid of the first master node in you cluster.
	ClusterID string `json:"clusterId"`
	// NodeName is the exact name that the virtual node will have.
	// One and only one of NodeName and NodeNamePrefix must be set.
	NodeName string `json:"nodeName,omitempty"`
	// NodeNamePrefix is the prefix that the virtual node will have.
	// One and only one of NodeName and NodeNamePrefix must be set.
	NodeNamePrefix string `json:"nodeNamePrefix,omitempty"`
	// Images is the list of the images already stored in the cluster.
	Images []corev1.ContainerImage `json:"images,omitempty"`
	// ResourceQuota contains the quantity of resources made available by the cluster.
	ResourceQuota corev1.ResourceQuotaSpec `json:"resourceQuota,omitempty"`
	// Labels contains the label to be added to the virtual node.
	Labels map[string]string `json:"labels,omitempty"`
	// Prices contains the possible prices for every kind of resource (cpu, memory, image).
	Prices corev1.ResourceList `json:"prices,omitempty"`
	// WithdrawalTimestamp is set when a graceful deletion is requested by the user.
	WithdrawalTimestamp *metav1.Time `json:"withdrawalTimestamp,omitempty"`
	// StorageClasses contains the list of the storage classes offered by the cluster.
	StorageClasses []StorageType `json:"storageClasses,omitempty"`
}

// OfferPhase describes the phase of the ResourceOffer.
type OfferPhase string

const (
	// ResourceOfferPending indicates a pending phase, an action is required.
	ResourceOfferPending OfferPhase = "Pending"
	// ResourceOfferManualActionRequired indicates that a manual action is required.
	ResourceOfferManualActionRequired OfferPhase = "ManualActionRequired"
	// ResourceOfferAccepted indicates an accepted offer.
	ResourceOfferAccepted OfferPhase = "Accepted"
	// ResourceOfferRefused indicates a refused offer.
	ResourceOfferRefused OfferPhase = "Refused"
)

// VirtualKubeletStatus indicates the observed status of the VirtualKubelet Deployment.
type VirtualKubeletStatus string

const (
	// VirtualKubeletStatusUnknown indicates that the VirtualKubelet Deployment status is unknown.
	VirtualKubeletStatusUnknown VirtualKubeletStatus = ""
	// VirtualKubeletStatusNone indicates that there is no VirtualKubelet Deployment.
	VirtualKubeletStatusNone VirtualKubeletStatus = "None"
	// VirtualKubeletStatusCreated indicates that the VirtualKubelet Deployment has been created.
	VirtualKubeletStatusCreated VirtualKubeletStatus = "Created"
	// VirtualKubeletStatusDeleting indicates that the VirtualKubelet Deployment is deleting.
	VirtualKubeletStatusDeleting VirtualKubeletStatus = "Deleting"
)

// ResourceOfferStatus defines the observed state of ResourceOffer.
type ResourceOfferStatus struct {
	// Phase is the status of this ResourceOffer.
	// When the offer is created it is checked by the operator, which sets this field to "Accepted" or "Refused" on tha base of cluster configuration.
	// If the ResourceOffer is accepted a virtual-kubelet for the foreign cluster will be created.
	// +kubebuilder:validation:Enum="Pending";"ManualActionRequired";"Accepted";"Refused"
	// +kubebuilder:default="Pending"
	Phase OfferPhase `json:"phase"`
	// VirtualKubeletStatus indicates if the virtual-kubelet for this ResourceOffer has been created or not.
	// +kubebuilder:validation:Enum="None";"Created";"Deleting"
	// +kubebuilder:default="None"
	VirtualKubeletStatus VirtualKubeletStatus `json:"virtualKubeletStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="offer",categories=liqo

// ResourceOffer is the Schema for the resourceOffers API.
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="VirtualKubeletStatus",type=string,JSONPath=`.status.virtualKubeletStatus`
// +kubebuilder:printcolumn:name="Local",type=string,JSONPath=`.metadata.labels.liqo\.io/replication`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ResourceOffer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceOfferSpec   `json:"spec,omitempty"`
	Status ResourceOfferStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ResourceOfferList contains a list of ResourceOffer.
type ResourceOfferList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceOffer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceOffer{}, &ResourceOfferList{})
}
