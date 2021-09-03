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
)

// ClusterConfigSpec defines the desired state of ClusterConfig.
type ClusterConfigSpec struct {
	// AdvertisementConfig defines the configuration for the advertisement protocol.
	DiscoveryConfig DiscoveryConfig `json:"discoveryConfig"`
}

// DiscoveryConfig defines the configuration of the Discovery logic.
type DiscoveryConfig struct {
	// ClusterName is a nickname for your cluster that can be easily understood by a user
	ClusterName string `json:"clusterName,omitempty"`

	// ClusterLabels is a set of labels which characterizes the local cluster when exposed remotely as a virtual node.
	// This field is deprecated and it is currently maintained for backward compatibility only.
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
	// This field is deprecated and it is currently maintained for backward compatibility only.
	// +kubebuilder:validation:Optional
	IncomingPeeringEnabled bool `json:"incomingPeeringEnabled"`

	AuthServiceAddress string `json:"authServiceAddress,omitempty"`
	AuthServicePort    string `json:"authServicePort,omitempty"`
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
