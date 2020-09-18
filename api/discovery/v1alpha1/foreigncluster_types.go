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
	advtypes "github.com/liqotech/liqo/api/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type DiscoveryType string

const (
	LanDiscovery             DiscoveryType = "LAN"
	WanDiscovery             DiscoveryType = "WAN"
	ManualDiscovery          DiscoveryType = "Manual"
	IncomingPeeringDiscovery DiscoveryType = "IncomingPeering"
)

// ForeignClusterSpec defines the desired state of ForeignCluster
type ForeignClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	ClusterIdentity  ClusterIdentity `json:"clusterIdentity"`
	Namespace        string          `json:"namespace"`
	Join             bool            `json:"join"`
	ApiUrl           string          `json:"apiUrl"`
	DiscoveryType    DiscoveryType   `json:"discoveryType"`
	AllowUntrustedCA bool            `json:"allowUntrustedCA"`
}

type ClusterIdentity struct {
	ClusterID   string `json:"clusterID"`
	ClusterName string `json:"clusterName,omitempty"`
}

// ForeignClusterStatus defines the observed state of ForeignCluster
type ForeignClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Outgoing Outgoing `json:"outgoing,omitempty"`
	Incoming Incoming `json:"incoming,omitempty"`
	Ttl      int      `json:"ttl,omitempty"`
}

type Outgoing struct {
	Joined                   bool                `json:"joined"`
	RemotePeeringRequestName string              `json:"remote-peering-request-name,omitempty"`
	CaDataRef                *v1.ObjectReference `json:"caDataRef,omitempty"`
	Advertisement            *v1.ObjectReference `json:"advertisement,omitempty"`
	AvailableIdentity        bool                `json:"availableIdentity,omitempty"`
	IdentityRef              *v1.ObjectReference `json:"identityRef,omitempty"`
	AdvertisementStatus      advtypes.AdvPhase   `json:"advertisementStatus,omitempty"`
}

type Incoming struct {
	Joined              bool                `json:"joined"`
	PeeringRequest      *v1.ObjectReference `json:"peeringRequest,omitempty"`
	AvailableIdentity   bool                `json:"availableIdentity,omitempty"`
	IdentityRef         *v1.ObjectReference `json:"identityRef,omitempty"`
	AdvertisementStatus advtypes.AdvPhase   `json:"advertisementStatus,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
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

	if err := AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	crdClient.AddToRegistry("foreignclusters", &ForeignCluster{}, &ForeignClusterList{}, nil, schema.GroupResource{
		Group:    GroupVersion.Group,
		Resource: "foreignclusters",
	})
}
