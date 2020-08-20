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

package v1

import (
	"github.com/liqoTech/liqo/pkg/crdClient"
	"github.com/liqoTech/liqo/pkg/liqonet"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

// ClusterConfigSpec defines the desired state of ClusterConfig
type ClusterConfigSpec struct {
	AdvertisementConfig AdvertisementConfig `json:"advertisementConfig"`
	DiscoveryConfig     DiscoveryConfig     `json:"discoveryConfig"`
	LiqonetConfig       LiqonetConfig       `json:"liqonetConfig"`
	DispatcherConfig    DispatcherConfig    `json:"dispatcherConfig,omitempty"`
}

type AdvertisementConfig struct {
	BroadcasterConfig `json:"broadcasterConfig,omitempty"`
	AdvOperatorConfig `json:"advOperatorConfig,omitempty"`
	// +kubebuilder:validation:Minimum=0
	KeepaliveThreshold int32 `json:"keepaliveThreshold,omitempty"`
	// +kubebuilder:validation:Minimum=0
	KeepaliveRetryTime int32 `json:"keepaliveRetryTime,omitempty"`
}

type BroadcasterConfig struct {
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	ResourceSharingPercentage int32 `json:"resourceSharingPercentage,omitempty"`
	EnableBroadcaster         bool  `json:"enableBroadcaster,omitempty"`
}

// AcceptPolicy defines the policy to accept/refuse an Advertisement
type AcceptPolicy string

const (
	// AutoAcceptAll means all the Advertisement received will be accepted
	AutoAcceptAll AcceptPolicy = "AutoAcceptAll"
	// AutoAcceptWithinMaximum means all the Advertisement received will be accepted until the MaxAcceptableAdvertisement limit is reached
	AutoAcceptWithinMaximum AcceptPolicy = "AutoAcceptWithinMaximum"
	// AutoRefuseAll means all the Advertisement received will be refused (but not deleted)
	AutoRefuseAll AcceptPolicy = "AutoRefuseAll"
	// ManualAccept means every Advertisement received will need a manual accept/refuse, which can be done by updating its status
	ManualAccept AcceptPolicy = "Manual"
)

type AdvOperatorConfig struct {
	// +kubebuilder:validation:Minimum=0
	MaxAcceptableAdvertisement int32 `json:"maxAcceptableAdvertisement,omitempty"`
	// +kubebuilder:validation:Enum="AutoAcceptAll";"AutoAcceptWithinMaximum";"AutoRefuseAll";"Manual"
	AcceptPolicy AcceptPolicy `json:"acceptPolicy"`
}

type DiscoveryConfig struct {
	// --- mDNS ---

	Name    string `json:"name"`
	Service string `json:"service"`
	Domain  string `json:"domain"`
	// +kubebuilder:validation:Maximum=65355
	// +kubebuilder:validation:Minimum=1
	Port int `json:"port"`

	// +kubebuilder:validation:Minimum=1
	WaitTime int `json:"waitTime"`
	// +kubebuilder:validation:Minimum=2
	UpdateTime int `json:"updateTime"`

	EnableDiscovery     bool `json:"enableDiscovery"`
	EnableAdvertisement bool `json:"enableAdvertisement"`

	AutoJoin          bool `json:"autojoin"`
	AutoJoinUntrusted bool `json:"autojoinUntrusted"`

	// --- DNS ---

	DnsServer string `json:"dnsServer"`

	// --- CA ---

	AllowUntrustedCA bool `json:"allowUntrustedCA"`
}

type LiqonetConfig struct {
	//contains a list of reserved subnets in CIDR notation used by the k8s cluster like the podCIDR and ClusterCIDR
	ReservedSubnets []string               `json:"reservedSubnets"`
	PodCIDR         string                 `json:"podCIDR"`
	VxlanNetConfig  liqonet.VxlanNetConfig `json:"vxlanNetConfig,omitempty"`
}

//contains a list of resources identified by their GVR
type Resource struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
}
type DispatcherConfig struct {
	ResourcesToReplicate []Resource `json:"resourcesToReplicate,omitempty"`
}

// ClusterConfigStatus defines the observed state of ClusterConfig
type ClusterConfigStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// ClusterConfig is the Schema for the clusterconfigs API
type ClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterConfigSpec   `json:"spec,omitempty"`
	Status ClusterConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterConfigList contains a list of ClusterConfig
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
	crdClient.AddToRegistry("clusterconfigs", &ClusterConfig{}, &ClusterConfigList{}, nil, schema.GroupResource{
		Group:    GroupVersion.Group,
		Resource: "clusterconfigs",
	})
}
