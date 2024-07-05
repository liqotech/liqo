// Copyright 2019-2024 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	liqov1alpha1 "github.com/liqotech/liqo/apis/core/v1alpha1"
)

// TenantResource is the name of the tenant resources.
var TenantResource = "tenants"

// TenantKind specifies the kind of the tenant.
var TenantKind = "Tenant"

// TenantGroupResource is group resource used to register these objects.
var TenantGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: TenantResource}

// TenantGroupVersionResource is groupResourceVersion used to register these objects.
var TenantGroupVersionResource = GroupVersion.WithResource(TenantResource)

// TenantSpec defines the desired state of Tenant.
type TenantSpec struct {
	// ClusterID is the id of the consumer cluster.
	ClusterID liqov1alpha1.ClusterID `json:"clusterID,omitempty"`
	// PublicKey is the public key of the tenant cluster.
	PublicKey []byte `json:"publicKey,omitempty"`
	// CSR is the Certificate Signing Request of the tenant cluster.
	CSR []byte `json:"csr,omitempty"`
	// Signature contains the nonce signed by the tenant cluster.
	Signature []byte `json:"signature,omitempty"`
	// ProxyURL is the URL of the proxy used by the tenant cluster to connect to the local cluster (optional).
	ProxyURL *string `json:"proxyURL,omitempty"`
	// TenantCondition contains the conditions of the tenant.
	// +kubebuilder:validation:Enum=Active;Cordoned;Drained
	// +kubebuilder:default=Active
	TenantCondition TenantCondition `json:"tenantCondition,omitempty"`
}

// TenantCondition contains the conditions of the tenant.
type TenantCondition string

const (
	// TenantConditionActive indicates that the tenant is active: it can consume resources and negotiate new ones.
	TenantConditionActive TenantCondition = "Active"
	// TenantConditionCordoned indicates that the tenant is cordoned: the existing resources are preserved (and cordoned),
	// but no new ones can be negotiated.
	TenantConditionCordoned TenantCondition = "Cordoned"
	// TenantConditionDrained indicates that the tenant is drained: it can't consume resources nor negotiate new ones.
	TenantConditionDrained TenantCondition = "Drained"
)

// TenantStatus defines the observed state of Tenant.
type TenantStatus struct {
	// TenantNamespace is the namespace of the tenant cluster.
	TenantNamespace string `json:"tenantNamespace,omitempty"`
	// AuthParams contains the authentication parameters for the consumer cluster.
	AuthParams *AuthParams `json:"authParams,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=liqo
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Condition",type=string,JSONPath=`.spec.tenantCondition`

// Tenant represents a consumer cluster.
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantList contains a list of Tenant.
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}
