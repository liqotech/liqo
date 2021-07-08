/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"

	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/discovery"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PeeringConditionStatusType indicates the phase of a peering with a remote cluster.
type PeeringConditionStatusType string

const (
	// PeeringConditionStatusNone indicates that there is no peering.
	PeeringConditionStatusNone PeeringConditionStatusType = "None"
	// PeeringConditionStatusPending indicates that the peering is pending,
	// and we are waiting for the remote cluster feedback.
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
)

// ForeignClusterSpec defines the desired state of ForeignCluster.
type ForeignClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foreign Cluster Identity.
	ClusterIdentity ClusterIdentity `json:"clusterIdentity,omitempty"`
	// Namespace where Liqo is deployed. (Deprecated)
	Namespace string `json:"namespace,omitempty"`
	// Enable join process to foreign cluster.
	// +kubebuilder:default=false
	Join bool `json:"join,omitempty"`
	// +kubebuilder:validation:Enum="LAN";"WAN";"Manual";"IncomingPeering"
	// +kubebuilder:default="Manual"
	// How this ForeignCluster has been discovered.
	DiscoveryType discovery.Type `json:"discoveryType,omitempty"`
	// URL where to contact foreign Auth service.
	AuthURL string `json:"authUrl"`
	// +kubebuilder:validation:Enum="Unknown";"Trusted";"Untrusted"
	// +kubebuilder:default="Unknown"
	// Indicates if this remote cluster is trusted or not.
	TrustMode discovery.TrustMode `json:"trustMode,omitempty"`
	// If discoveryType is LAN or WAN and this indicates the number of seconds after that
	// this ForeignCluster will be removed if no updates have been received.
	// +kubebuilder:validation:Minimum=0
	TTL int `json:"ttl,omitempty"`
}

// ClusterIdentity contains the information about a remote cluster (ID and Name).
type ClusterIdentity struct {
	// Foreign Cluster ID, this is a unique identifier of that cluster.
	ClusterID string `json:"clusterID"`
	// Foreign Cluster Name to be shown in GUIs.
	ClusterName string `json:"clusterName,omitempty"`
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
	// AuthenticationStatusCondition informs users about the Authentication status.
	AuthenticationStatusCondition PeeringConditionType = "AuthenticationStatus"
)

// PeeringCondition contains details about state of the peering.
type PeeringCondition struct {
	// Type of the peering condition.
	// +kubebuilder:validation:Enum="OutgoingPeering";"IncomingPeering";"NetworkStatus";"AuthenticationStatus"
	Type PeeringConditionType `json:"type"`
	// Status of the condition.
	// +kubebuilder:validation:Enum="None";"Pending";"Established";"Disconnecting";"Denied";"EmptyDenied"
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
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status

// ForeignCluster is the Schema for the foreignclusters API.
// +kubebuilder:printcolumn:name="Outgoing peering phase",type=string,JSONPath=`.status.peeringConditions[?(@.type == 'OutgoingPeering')].status`
// +kubebuilder:printcolumn:name="Incoming peering phase",type=string,JSONPath=`.status.peeringConditions[?(@.type == 'IncomingPeering')].status`
// +kubebuilder:printcolumn:name="Networking status",type=string,JSONPath=`.status.peeringConditions[?(@.type == 'NetworkStatus')].status`
// +kubebuilder:printcolumn:name="Authentication status",type=string,JSONPath=`.status.peeringConditions[?(@.type == 'AuthenticationStatus')].status`
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

	if err := AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	crdclient.AddToRegistry("foreignclusters", &ForeignCluster{}, &ForeignClusterList{}, nil, ForeignClusterGroupResource)
}
