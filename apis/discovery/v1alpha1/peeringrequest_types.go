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
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	object_references "github.com/liqotech/liqo/pkg/object-references"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PeeringRequestSpec defines the desired state of PeeringRequest
type PeeringRequestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foreign Cluster Identity
	ClusterIdentity ClusterIdentity `json:"clusterIdentity"`
	// Namespace where Liqo is deployed
	Namespace string `json:"namespace"`
	// KubeConfig file (with Advertisement creation role) secret reference
	KubeConfigRef *v1.ObjectReference `json:"kubeConfigRef,omitempty"`
	// Local auth service address
	AuthUrl string `json:"authUrl"`
}

// PeeringRequestStatus defines the observed state of PeeringRequest
type PeeringRequestStatus struct {
	BroadcasterRef      *object_references.DeploymentReference `json:"broadcasterRef,omitempty"`
	AdvertisementStatus advtypes.AdvPhase                      `json:"advertisementStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// PeeringRequest is the Schema for the PeeringRequests API
type PeeringRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PeeringRequestSpec   `json:"spec,omitempty"`
	Status PeeringRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PeeringRequestList contains a list of PeeringRequest
type PeeringRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PeeringRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PeeringRequest{}, &PeeringRequestList{})

	if err := AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	crdClient.AddToRegistry("peeringrequests", &PeeringRequest{}, &PeeringRequestList{}, nil, schema.GroupResource{
		Group:    v1.SchemeGroupVersion.Group,
		Resource: "peeringrequests",
	})
}
