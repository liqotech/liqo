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
}

type AdvertisementConfig struct {
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Minimum=0
	ResourceSharingPercentage int32 `json:"resourceSharingPercentage,omitempty"`
	EnableBroadcaster         bool  `json:"enableBroadcaster,omitempty"`
	// +kubebuilder:validation:Minimum=0
	MaxAcceptableAdvertisement int32 `json:"maxAcceptableAdvertisement,omitempty"`
	AutoAccept                 bool  `json:"autoAccept"`
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
	ReservedSubnets  []string               `json:"reservedSubnets"`
	GatewayPrivateIP string                 `json:"gatewayPrivateIP"`
	VxlanNetConfig   liqonet.VxlanNetConfig `json:"vxlanNetConfig,omitempty"`
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
