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
	object_references "github.com/liqoTech/liqo/pkg/object-references"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkInfo struct {
	PodCIDR          string `json:"podCIDR"`
	GatewayIP        string `json:"gatewayIP"`
	GatewayPrivateIP string `json:"gatewayPrivateIP"`
	// +optional
	SupportedProtocols []string `json:"supportedProtocols,omitempty"`
}

type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// AdvertisementSpec defines the desired state of Advertisement
type AdvertisementSpec struct {
	ClusterId     string                                      `json:"clusterId"`
	Images        []corev1.ContainerImage                     `json:"images,omitempty"`
	LimitRange    corev1.LimitRangeSpec                       `json:"limitRange,omitempty"`
	ResourceQuota corev1.ResourceQuotaSpec                    `json:"resourceQuota,omitempty"`
	Neighbors     map[corev1.ResourceName]corev1.ResourceList `json:"neighbors,omitempty"`
	Properties    map[corev1.ResourceName]string              `json:"properties,omitempty"`
	Prices        corev1.ResourceList                         `json:"prices,omitempty"`
	Network       NetworkInfo                                 `json:"network"`
	KubeConfigRef corev1.SecretReference                      `json:"kubeConfigRef"`
	Timestamp     metav1.Time                                 `json:"timestamp"`
	TimeToLive    metav1.Time                                 `json:"timeToLive"`
}

// AdvertisementStatus defines the observed state of Advertisement
type AdvertisementStatus struct {
	AdvertisementStatus   string                                `json:"advertisementStatus"`
	VkCreated             bool                                  `json:"vkCreated"`
	VkReference           object_references.DeploymentReference `json:"vkReference,omitempty"`
	LocalRemappedPodCIDR  string                                `json:"localRemappedPodCIDR,omitempty"`
	RemoteRemappedPodCIDR string                                `json:"remoteRemappedPodCIDR,omitempty"`
	//the tunnelEndpoint associated with the foreign cluster
	TunnelEndpointKey NamespacedName `json:"tunnelEndpointKey"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName="adv"
// +kubebuilder:resource:scope=Cluster

// Advertisement is the Schema for the advertisements API
type Advertisement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AdvertisementSpec   `json:"spec,omitempty"`
	Status AdvertisementStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AdvertisementList contains a list of Advertisement
type AdvertisementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Advertisement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Advertisement{}, &AdvertisementList{})
}
