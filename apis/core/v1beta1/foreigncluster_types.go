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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterID contains the unique identifier of a ForeignCluster. It must be a DNS (RFC 1123) compatible name.
// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
type ClusterID string

// ForeignClusterSpec defines the desired state of ForeignCluster.
type ForeignClusterSpec struct {
	// Foreign Cluster ID.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="ClusterID field is immutable"
	ClusterID ClusterID `json:"clusterID"`
}

// RoleType represents the role of a ForeignCluster.
type RoleType string

// These are valid roles for a ForeignCluster.
const (
	// ConsumerRole represents a cluster that consumes resources from the local cluster.
	ConsumerRole RoleType = "Consumer"
	// ProviderRole represents a cluster that provides resources to the local cluster.
	ProviderRole RoleType = "Provider"
	// ConsumerAndProviderRole represents a cluster that consumes and provides resources to the local cluster.
	ConsumerAndProviderRole RoleType = "ConsumerAndProvider"
	// UnknownRole represents a cluster whose role is unknown.
	UnknownRole RoleType = "Unknown"
)

// ForeignClusterStatus defines the observed state of ForeignCluster.
type ForeignClusterStatus struct {
	// Role of the ForeignCluster.
	// +kubebuilder:validation:Enum="Consumer";"Provider";"ConsumerAndProvider";"Unknown"
	// +kubebuilder:default="Unknown"
	Role RoleType `json:"role"`

	// Modules contains the configuration of the modules for this foreign cluster.
	Modules Modules `json:"modules,omitempty"`

	// URL of the forign cluster's API server.
	// +kubebuilder:validation:Optional
	APIServerURL string `json:"apiServerUrl,omitempty"`

	// URL where to contact foreign proxy for the api server.
	// This URL is used when creating the k8s clients toward the remote cluster.
	// +kubebuilder:validation:Optional
	ForeignProxyURL string `json:"foreignProxyUrl,omitempty"`

	// TenantNamespace names in the peered clusters
	// +kubebuilder:validation:Optional
	TenantNamespace TenantNamespaceType `json:"tenantNamespace"`

	// Generic conditions related to the foreign cluster.
	Conditions []Condition `json:"conditions,omitempty"`
}

// Modules contains the configuration of the modules for this foreign cluster.
type Modules struct {
	Networking     Module `json:"networking"`
	Authentication Module `json:"authentication"`
	Offloading     Module `json:"offloading"`
}

// Module contains the configuration and conditions of a module for a foreign cluster.
type Module struct {
	// Enabled indicates if the module is enabled or not.
	Enabled bool `json:"enabled"`
	// Conditions contains the status conditions related to the module.
	Conditions []Condition `json:"conditions,omitempty"`
}

// ConditionType represents different conditions that a  could assume.
type ConditionType string

// These are valid type of Conditions.
const (
	// GENERIC
	// APIServerStatusCondition shows the status of the API Server.
	APIServerStatusCondition ConditionType = "APIServerStatus"

	// NETWORKING
	// NetworkConfigurationStatusCondition tells whether the network configuration of the peer cluster is present.
	NetworkConfigurationStatusCondition ConditionType = "NetworkConfigurationStatus"
	// NetworkConnectionStatusCondition shows the network connection status.
	NetworkConnectionStatusCondition ConditionType = "NetworkConnectionStatus"
	NetworkGatewayPresenceCondition  ConditionType = "NetworkGatewayPresence"
	// NetworkGatewayServerStatusCondition shows the network gateway server status.
	NetworkGatewayServerStatusCondition ConditionType = "NetworkGatewayServerStatus"
	// NetworkGatewayClientStatusCondition shows the network gateway client status.
	NetworkGatewayClientStatusCondition ConditionType = "NetworkGatewayClientStatus"

	// AUTHENTICATION
	// AuthIdentityControlPlaneStatusCondition shows the status of the ControlPlane Identity.
	AuthIdentityControlPlaneStatusCondition ConditionType = "AuthIdentityControlPlaneStatus"
	// AuthTenantStatusCondition shows the status of the Tenant.
	AuthTenantStatusCondition ConditionType = "AuthTenantStatus"

	// OFFLOADING
	// OffloadingVirtualNodeStatusCondition shows the status of a Virtual Node.
	OffloadingVirtualNodeStatusCondition ConditionType = "OffloadingVirtualNodeStatus"
	// OffloadingNodeStatusCondition shows the status of a Node.
	OffloadingNodeStatusCondition ConditionType = "OffloadingNodeStatus"
)

// ConditionStatusType indicates the status of a condition with a remote cluster.
type ConditionStatusType string

const (
	// ConditionStatusNone indicates that the condition is not applicable.
	ConditionStatusNone ConditionStatusType = "None"
	// ConditionStatusPending indicates that the condition is pending.
	ConditionStatusPending ConditionStatusType = "Pending"
	// ConditionStatusEstablished indicates that the condition has been established.
	ConditionStatusEstablished ConditionStatusType = "Established"
	// ConditionStatusError indicates that an error has occurred.
	ConditionStatusError ConditionStatusType = "Error"
	// ConditionStatusReady indicates that the condition is ready.
	ConditionStatusReady ConditionStatusType = "Ready"
	// ConditionStatusNotReady indicates that the condition is not ready.
	ConditionStatusNotReady ConditionStatusType = "NotReady"
	// ConditionStatusSomeNotReady indicates that not all components of the conditions are ready.
	ConditionStatusSomeNotReady ConditionStatusType = "SomeNotReady"
)

// Condition contains details about state of a.
type Condition struct {
	// Type of the condition.
	// +kubebuilder:validation:Enum="APIServerStatus";"NetworkConnectionStatus";"NetworkGatewayServerStatus";"NetworkGatewayClientStatus";"NetworkGatewayPresence";"NetworkConfigurationStatus";"AuthIdentityControlPlaneStatus";"AuthTenantStatus";"OffloadingVirtualNodeStatus";"OffloadingNodeStatus"
	//
	//nolint:lll // ignore long lines given by Kubebuilder marker annotations
	Type ConditionType `json:"type"`
	// Status of the condition.
	// +kubebuilder:validation:Enum="None";"Pending";"Established";"Error";"Ready";"NotReady";"SomeNotReady"
	// +kubebuilder:default="None"
	Status ConditionStatusType `json:"status"`
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
// +kubebuilder:resource:scope=Cluster,categories=liqo,shortName=fc
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=`.status.role`
// +kubebuilder:printcolumn:name="ClusterID",type=string,priority=1,JSONPath=`.spec.clusterID`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ForeignCluster is the Schema for the foreignclusters API.
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
