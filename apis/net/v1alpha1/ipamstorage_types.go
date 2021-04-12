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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// IpamSpec defines the desired state of Ipam
type IpamSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// Map consumed by go-ipam module. Key is prefic cidr, value is a Prefix
	Prefixes map[string][]byte `json:"prefixes"`
	// Network pools
	Pools []string `json:"pools"`
	// Map used to keep track of networks assigned to clusters. Key is the cluster, value is the network.
	ClusterSubnet map[string]string `json:"clusterSubnet"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// Ipam is the Schema for the ipams API
type IpamStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec IpamSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// IpamList contains a list of Ipam
type IpamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IpamStorage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IpamStorage{}, &IpamList{})
}
