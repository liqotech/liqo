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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"

	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/labelPolicy"
)

// ClusterConfigSpec defines the desired state of ClusterConfig.
type ClusterConfigSpec struct {
	APIServerConfig APIServerConfig `json:"apiServerConfig,omitempty"`
	// AdvertisementConfig defines the configuration for the advertisement protocol.
	AdvertisementConfig AdvertisementConfig `json:"resourceSharingConfig"`
	DiscoveryConfig     DiscoveryConfig     `json:"discoveryConfig"`
	AuthConfig          AuthConfig          `json:"authConfig"`
	LiqonetConfig       LiqonetConfig       `json:"liqonetConfig"`
}

// +kubebuilder:validation:Pattern="^([0-9]{1,3}.){3}[0-9]{1,3}(/([0-9]|[1-2][0-9]|3[0-2]))$"
type CIDR string

// AdvertisementConfig defines the configuration for the advertisement protocol.
type AdvertisementConfig struct {
	// OutgoingConfig defines the behavior for the creation of Advertisements on other clusters.
	OutgoingConfig BroadcasterConfig `json:"outgoingConfig"`
	// IngoingConfig defines the behavior for the acceptance of Advertisements from other clusters.
	IngoingConfig AdvOperatorConfig `json:"ingoingConfig,omitempty"`
	// KeepaliveThreshold defines the number of failed attempts to contact the foreign cluster your cluster will
	// tolerate before deleting it.
	// +kubebuilder:validation:Minimum=0
	KeepaliveThreshold int32 `json:"keepaliveThreshold,omitempty"`
	// After establishing a sharing with a foreign cluster, a keepalive mechanism starts, in order to know if the
	// foreign cluster is reachable or not.
	// KeepaliveRetryTime defines the time between an attempt to contact the foreign cluster and the next one.
	// +kubebuilder:validation:Minimum=0
	KeepaliveRetryTime int32 `json:"keepaliveRetryTime,omitempty"`
	// LabelPolicies contains the policies for each label to be added to remote virtual nodes.
	LabelPolicies []LabelPolicy `json:"labelPolicies,omitempty"`
}

// BroadcasterConfig defines the configuration for the broadcasting protocol.
type BroadcasterConfig struct {
	// ResourceSharingPercentage defines the percentage of your cluster resources that you will share with foreign
	// clusters.
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	ResourceSharingPercentage int32 `json:"resourceSharingPercentage"`
}

// AcceptPolicy defines the policy to accept/refuse an Advertisement.
type AcceptPolicy string

const (
	// AutoAcceptMax means all the Advertisement received will be accepted until the MaxAcceptableAdvertisement limit is
	// reached. AutoAcceptAll can be achieved by setting MaxAcceptableAdvertisement to 1000000.
	// AutoRefuseAll can be achieved by setting MaxAcceptableAdvertisement to 0.
	AutoAcceptMax AcceptPolicy = "AutoAcceptMax"
	// ManualAccept means every Advertisement received will need a manual accept/refuse, which can be done by updating
	// its status.
	ManualAccept AcceptPolicy = "Manual"
)

// AdvOperatorConfig defines the configuration of the AdvertisementOperator.
type AdvOperatorConfig struct {
	// MaxAcceptableAdvertisement defines the maximum number of Advertisements that can be accepted over time.
	// The maximum value for this field is set to 1000000, a symbolic value that implements the AcceptAll policy.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000000
	MaxAcceptableAdvertisement int32 `json:"maxAcceptableAdvertisement"`
	// AcceptPolicy defines the policy to accept/refuse an Advertisement.
	// Possible values are AutoAcceptMax and Manual.
	// AutoAcceptMax means all the Advertisement received will be accepted until the MaxAcceptableAdvertisement limit
	// is reached; Manual means every Advertisement received will need a manual accept/refuse, which can be done by
	// updating its status.
	// +kubebuilder:validation:Enum="AutoAcceptMax";"Manual"
	AcceptPolicy AcceptPolicy `json:"acceptPolicy"`
}

// LabelPolicy define a key-value structure to indicate which keys have to be aggregated and with which policy.
type LabelPolicy struct {
	// Label Key to be aggregated in new virtual nodes
	Key string `json:"key"`
	// Merge labels Policy
	// +kubebuilder:validation:Enum="LabelPolicyAnyTrue";"LabelPolicyAllTrue";"LabelPolicyAnyTrueNoLabelIfFalse";"LabelPolicyAllTrueNoLabelIfFalse"
	// +kubebuilder:default="LabelPolicyAnyTrue"
	Policy labelPolicy.LabelPolicyType `json:"policy,omitempty"`
}

// APIServerConfig defines the configuration of the cluster APIServer.
type APIServerConfig struct {
	Address   string `json:"address,omitempty"`
	TrustedCA bool   `json:"trustedCA,omitempty"`
}

// DiscoveryConfig defines the configuration of the Discovery logic.
type DiscoveryConfig struct {
	// ClusterName is a nickname for your cluster that can be easily understood by a user
	ClusterName string `json:"clusterName,omitempty"`

	// ClusterLabels is a set of labels which characterizes the local cluster when exposed remotely as a virtual node.
	ClusterLabels map[string]string `json:"clusterLabels,omitempty"`
	// --- mDNS ---

	Name    string `json:"name"`
	Service string `json:"service"`
	// +kubebuilder:default="_liqo_auth._tcp"
	AuthService string `json:"authService,omitempty"`
	Domain      string `json:"domain"`
	// +kubebuilder:validation:Maximum=65355
	// +kubebuilder:validation:Minimum=1
	Port int `json:"port"`
	// +kubebuilder:validation:Minimum=30
	TTL uint32 `json:"ttl"`

	EnableDiscovery     bool `json:"enableDiscovery"`
	EnableAdvertisement bool `json:"enableAdvertisement"`

	AutoJoin bool `json:"autojoin"`

	// Allow (by default) the remote clusters to establish a peering with our cluster.
	// +kubebuilder:validation:Optional
	IncomingPeeringEnabled bool `json:"incomingPeeringEnabled"`

	AuthServiceAddress string `json:"authServiceAddress,omitempty"`
	AuthServicePort    string `json:"authServicePort,omitempty"`
}

// PeeringPermission collects the list of ClusterRoles to be attributed to foreign cluster in the different steps of
// peering.
type PeeringPermission struct {
	// The list of ClusterRoles to be enabled with the creation of the Tenant Namespace, these ClusterRoles
	// have the basic permissions to give to a remote cluster
	Basic []string `json:"basic,omitempty"`
	// The list of ClusterRoles be enabled when a ResourceRequest has been accepted, these ClusterRoles have the
	// permissions required to a remote cluster to manage an outgoing peering (incoming for the local cluster), when the
	// Pods will be offloaded to the local cluster
	Incoming []string `json:"incoming,omitempty"`
	// The list of ClusterRoles to be enabled when we send a ResourceRequest, these ClusterRoles have the permissions
	// required to a remote cluster to manage an incoming peering (outgoing for the local cluster), when the Pods will
	// be offloaded from the local cluster
	Outgoing []string `json:"outgoing,omitempty"`
}

// AuthConfig defines the configuration of the Authentication Server.
type AuthConfig struct {
	// Ask to remote clusters to provide a token to obtain an identity.
	// +kubebuilder:default=true
	// +kubebuilder:validation:Optional
	EnableAuthentication *bool `json:"enableAuthentication"`

	// Set the ClusterRoles to bind in the different peering stages
	PeeringPermission *PeeringPermission `json:"peeringPermission,omitempty"`
}

// LiqonetConfig defines the configuration of the Liqo Networking.
type LiqonetConfig struct {
	// This field is used by the IPAM embedded in the tunnelEndpointCreator.
	// Subnets listed in this field are excluded from the list of possible subnets used for natting POD CIDR.
	// Add here the subnets already used in your environment as a list in CIDR notation
	// (e.g. [10.1.0.0/16, 10.200.1.0/24]).
	ReservedSubnets []CIDR `json:"reservedSubnets"`
	// The subnet used by the cluster for the pods, in CIDR notation
	// +kubebuilder:validation:Pattern="^([0-9]{1,3}.){3}[0-9]{1,3}(/([0-9]|[1-2][0-9]|3[0-2]))$"
	PodCIDR string `json:"podCIDR"`
	// The subnet used by the cluster for the services, in CIDR notation
	// +kubebuilder:validation:Pattern="^([0-9]{1,3}.){3}[0-9]{1,3}(/([0-9]|[1-2][0-9]|3[0-2]))$"
	ServiceCIDR string `json:"serviceCIDR"`
	// Set of additional user-defined network pools.
	// Default set of network pools is: [192.168.0.0/16, 10.0.0.0/8, 172.16.0.0/12]
	AdditionalPools []CIDR `json:"additionalPools"`
}

// ClusterConfigStatus defines the observed state of ClusterConfig.
type ClusterConfigStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ClusterConfig is the Schema for the clusterconfigs API.
type ClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterConfigSpec   `json:"spec,omitempty"`
	Status ClusterConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterConfigList contains a list of ClusterConfig.
type ClusterConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterConfig{}, &ClusterConfigList{})

	if err := AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	crdclient.AddToRegistry("clusterconfigs", &ClusterConfig{}, &ClusterConfigList{},
		nil, schema.GroupResource{
			Group:    GroupVersion.Group,
			Resource: "clusterconfigs",
		})
}
