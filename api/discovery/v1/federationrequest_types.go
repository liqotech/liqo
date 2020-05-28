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

// FederationRequestSpec defines the desired state of FederationRequest
type FederationRequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ClusterID  string `json:"clusterID"`
	KubeConfig string `json:"kubeConfig"`
}

// FederationRequestStatus defines the observed state of FederationRequest
type FederationRequestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// FederationRequest is the Schema for the FederationRequests API
type FederationRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FederationRequestSpec   `json:"spec,omitempty"`
	Status FederationRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// FederationRequestList contains a list of FederationRequest
type FederationRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FederationRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FederationRequest{}, &FederationRequestList{})

	v1alpha1.AddToRegistry("federationrequest", &FederationRequest{})
	v1alpha1.AddToRegistry("federationrequests", &FederationRequestList{})
}
