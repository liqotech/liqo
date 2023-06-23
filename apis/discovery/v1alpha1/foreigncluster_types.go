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

// Package v1alpha1 contains API Schema definitions for the discovery v1alpha1 API group
//
//nolint:lll // ignore long lines given by Kubebuilder marker annotations
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PeeringConditionStatusType indicates the phase of a peering with a remote cluster.
type PeeringConditionStatusType string

const (
	// PeeringConditionStatusNone indicates that there is no peering.
	PeeringConditionStatusNone PeeringConditionStatusType = "None"
	// PeeringConditionStatusPending indicates that the peering is pending,
	// and we are either waiting for the remote cluster feedback or for us to accept the ResourceOffer.
	PeeringConditionStatusPending PeeringConditionStatusType = "Pending"
	// PeeringConditionStatusEstablished indicates that the peering has been established.
	PeeringConditionStatusEstablished PeeringConditionStatusType = "Established"
	// PeeringConditionStatusDisconnecting indicates that the peering is being deleted.
	PeeringConditionStatusDisconnecting PeeringConditionStatusType = "Disconnecting"
	// PeeringConditionStatusDenied indicates that the condition has been denied.
	// This is only used by the AuthenticationCondition Type, and indicates that
	// the authentication has been denied even if we provided a token.
	PeeringConditionStatusDenied PeeringConditionStatusType = "Denied"
	// PeeringConditionStatusEmptyDenied indicates that the condition has been denied.
	// This is only used by the AuthenticationCondition Type, and indicates that
	// the identity verification was denied with an empty token.
	PeeringConditionStatusEmptyDenied PeeringConditionStatusType = "EmptyDenied"
	// PeeringConditionStatusError indicates that an error has occurred.
	PeeringConditionStatusError PeeringConditionStatusType = "Error"
	// PeeringConditionStatusSuccess indicates that the condition is successful.
	PeeringConditionStatusSuccess PeeringConditionStatusType = "Success"
	// PeeringConditionStatusExternal indicates that the condition is managed by an external component.
	PeeringConditionStatusExternal PeeringConditionStatusType = "External"
)

// PeeringType defines the type of peering to be established.
type PeeringType string

const (
	// PeeringTypeOutOfBand represents an out-of-band control-plane peering
	// (i.e., control plane traffic flows outside the network fabric).
	PeeringTypeOutOfBand PeeringType = "OutOfBand"
	// PeeringTypeInBand represents an in-band control-plane peering.
	// (i.e., control plane traffic flows inside the network fabric).
	PeeringTypeInBand PeeringType = "InBand"
	// PeeringTypeUnknown represents the empty value for the peering type.
	PeeringTypeUnknown PeeringType = ""
)

// PeeringEnabledType indicates the desired state for the peering with this remote cluster.
type PeeringEnabledType string

const (
	// PeeringEnabledAuto indicates to use the default settings for the discovery method.
	// This is useful to track that the user did not set the peering state for that cluster,
	// if the peering is Auto liqo will use the default for that discovery method:
	// manual -> No.
	// incomingPeering -> No.
	// LAN -> Yes.
	PeeringEnabledAuto PeeringEnabledType = "Auto"
	// PeeringEnabledNo indicates to disable the peering with this remote cluster.
	PeeringEnabledNo PeeringEnabledType = "No"
	// PeeringEnabledYes indicates to enable the peering with this remote cluster.
	PeeringEnabledYes PeeringEnabledType = "Yes"
)

// ForeignClusterSpec defines the desired state of ForeignCluster.
type ForeignClusterSpec struct {
	// The type of peering to be established.
	// +kubebuilder:validation:Enum="OutOfBand";"InBand"
	// +kubebuilder:default="OutOfBand"
	// +kubebuilder:validation:Optional
	PeeringType PeeringType `json:"peeringType,omitempty"`

	// Foreign Cluster Identity.
	ClusterIdentity ClusterIdentity `json:"clusterIdentity,omitempty"`
	// Enable the peering process to the remote cluster.
	// +kubebuilder:validation:Enum="Auto";"No";"Yes"
	// +kubebuilder:default="Auto"
	// +kubebuilder:validation:Optional
	OutgoingPeeringEnabled PeeringEnabledType `json:"outgoingPeeringEnabled"`
	// Allow the remote cluster to establish a peering with our cluster.
	// +kubebuilder:validation:Enum="Auto";"No";"Yes"
	// +kubebuilder:default="Auto"
	// +kubebuilder:validation:Optional
	IncomingPeeringEnabled PeeringEnabledType `json:"incomingPeeringEnabled"`
	// URL where to contact foreign Auth service.
	// +kubebuilder:validation:Pattern=`https:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)`
	ForeignAuthURL string `json:"foreignAuthUrl"`
	// URL where to contact foreign proxy for the api server. This URL is used when
	// creating the k8s clients toward the remote cluster.
	ForeignProxyURL string `json:"foreignProxyUrl,omitempty"`
	// Indicates if the local cluster has to skip the tls verification over the remote Authentication Service or not.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	InsecureSkipTLSVerify *bool `json:"insecureSkipTLSVerify"`
	// If discoveryType is LAN, this indicates the number of seconds after that
	// this ForeignCluster will be removed if no updates have been received.
	// +kubebuilder:validation:Minimum=0
	TTL int `json:"ttl,omitempty"`
}

// ClusterIdentity contains the information about a remote cluster (ID and Name).
type ClusterIdentity struct {
	// Foreign Cluster ID, this is a unique identifier of that cluster.
	ClusterID string `json:"clusterID"`
	// Foreign Cluster Name to be shown in GUIs.
	ClusterName string `json:"clusterName"`
}

// String returns the ClusterName. It makes it possible to format ClusterIdentities with %s.
func (i ClusterIdentity) String() string {
	return i.ClusterName
}

// ForeignClusterStatus defines the observed state of ForeignCluster.
type ForeignClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// TenantNamespace names in the peered clusters
	// +kubebuilder:validation:Optional
	TenantNamespace TenantNamespaceType `json:"tenantNamespace"`

	// PeeringConditions contains the conditions about the peering related to this
	// ForeignCluster.
	PeeringConditions []PeeringCondition `json:"peeringConditions,omitempty"`

	// URL of the forign cluster's API server.
	// +kubebuilder:validation:Optional
	APIServerURL string `json:"apiServerUrl,omitempty"`
}

// PeeringConditionType represents different conditions that a peering could assume.
type PeeringConditionType string

// These are valid conditions of a peering.
const (
	// OutgoingPeeringCondition informs users about the outgoing peering status.
	OutgoingPeeringCondition PeeringConditionType = "OutgoingPeering"
	// IncomingPeeringCondition informs users about the incoming peering status.
	IncomingPeeringCondition PeeringConditionType = "IncomingPeering"
	// NetworkStatusCondition informs users about the network status.
	NetworkStatusCondition PeeringConditionType = "NetworkStatus"
	// APIServerStatusCondition informs users about the foreign API server status.
	APIServerStatusCondition PeeringConditionType = "APIServerStatus"
	// AuthenticationStatusCondition informs users about the Authentication status.
	AuthenticationStatusCondition PeeringConditionType = "AuthenticationStatus"
	// ProcessForeignClusterStatusCondition informs users whether the Foreign Cluster is processable.
	ProcessForeignClusterStatusCondition PeeringConditionType = "ProcessForeignClusterStatus"
)

// PeeringCondition contains details about state of the peering.
type PeeringCondition struct {
	// Type of the peering condition.
	// +kubebuilder:validation:Enum="OutgoingPeering";"IncomingPeering";"NetworkStatus";"APIServerStatus";"AuthenticationStatus";"ProcessForeignClusterStatus"
	Type PeeringConditionType `json:"type"`
	// Status of the condition.
	// +kubebuilder:validation:Enum="None";"Pending";"Established";"Disconnecting";"Denied";"EmptyDenied";"Error";"Success";"External"
	// +kubebuilder:default="None"
	Status PeeringConditionStatusType `json:"status"`
	// LastTransitionTime -> timestamp for when the condition last transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason -> Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message -> Human-readable message indicating details about the last status transition.
	Message string `json:"message,omitempty"`
}

// TenantNamespaceType contains the names of the local and the remote
// namespaces assigned to the pair of clusters.
type TenantNamespaceType struct {
	// local TenantNamespace name
	Local string `json:"local,omitempty"`
	// remote TenantNamespace name
	Remote string `json:"remote,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=liqo
// +kubebuilder:subresource:status

// ForeignCluster is the Schema for the foreignclusters API.
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.peeringType`
// +kubebuilder:printcolumn:name="ClusterID",type=string,priority=1,JSONPath=`.spec.clusterIdentity.clusterID`
// +kubebuilder:printcolumn:name="ClusterName",type=string,priority=1,JSONPath=`.spec.clusterIdentity.clusterName`
// +kubebuilder:printcolumn:name="Outgoing peering",type=string,JSONPath=`.status.peeringConditions[?(@.type == 'OutgoingPeering')].status`
// +kubebuilder:printcolumn:name="Incoming peering",type=string,JSONPath=`.status.peeringConditions[?(@.type == 'IncomingPeering')].status`
// +kubebuilder:printcolumn:name="Networking",type=string,JSONPath=`.status.peeringConditions[?(@.type == 'NetworkStatus')].status`
// +kubebuilder:printcolumn:name="API Server",type=string,priority=1,JSONPath=`.status.peeringConditions[?(@.type == 'APIServerStatus')].status`
// +kubebuilder:printcolumn:name="Authentication",type=string,JSONPath=`.status.peeringConditions[?(@.type == 'AuthenticationStatus')].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ForeignCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ForeignClusterSpec   `json:"spec,omitempty"`
	Status ForeignClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ForeignClusterList contains a list of ForeignCluster.
type ForeignClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ForeignCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ForeignCluster{}, &ForeignClusterList{})
}
