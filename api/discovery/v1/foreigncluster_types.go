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
	"github.com/netgroup-polito/dronev2/pkg/crdClient/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ForeignClusterSpec defines the desired state of ForeignCluster
type ForeignClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ClusterID  string `json:"clusterID"`
	KubeConfig string `json:"kubeConfig"`
	Federate   bool   `json:"federate"`
}

// ForeignClusterStatus defines the observed state of ForeignCluster
type ForeignClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Federated bool `json:"federated"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ForeignCluster is the Schema for the foreignclusters API
type ForeignCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ForeignClusterSpec   `json:"spec,omitempty"`
	Status ForeignClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ForeignClusterList contains a list of ForeignCluster
type ForeignClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ForeignCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ForeignCluster{}, &ForeignClusterList{})

	v1alpha1.AddToRegistry("foreigncluster", &ForeignCluster{})
	v1alpha1.AddToRegistry("foreignclusters", &ForeignClusterList{})
}
