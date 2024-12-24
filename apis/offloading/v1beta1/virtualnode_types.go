// Copyright 2019-2025 The Liqo Authors
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

package v1beta1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

// Affinity contains the affinity and anti-affinity rules for the virtual node.
type Affinity struct {
	NodeAffinity *corev1.NodeAffinity `json:"nodeAffinity,omitempty"`
}

// OffloadingPatch contains the information to patch the virtual node.
type OffloadingPatch struct {
	// AnnotationsNotReflected is the list of annotations (key) that must not be reflected
	AnnotationsNotReflected []string `json:"annotationsNotReflected,omitempty"`
	// LabelsNotReflected is the list of labels (key) that must not be reflected
	LabelsNotReflected []string `json:"labelsNotReflected,omitempty"`
	// NodeSelector contains the node selector to target the remote cluster.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations contains the tolerations to target the remote cluster.
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// Affinity contains the affinity and anti-affinity rules to target the remote cluster.
	Affinity *Affinity `json:"affinity,omitempty"`
	// RuntimeClassName contains the runtimeclass name the pods should have on the target remote cluster.
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`
}

// DeploymentTemplate contains the deployment template of the virtual node.
type DeploymentTemplate struct {
	// Metadata contains the metadata of the virtual node.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the deployment spec of the virtual node.
	Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
}

// VirtualNodeSpec defines the desired state of VirtualNode.
type VirtualNodeSpec struct {
	// ClusterID contains the id of the remote cluster targeted by the created virtualKubelet.
	ClusterID liqov1beta1.ClusterID `json:"clusterID,omitempty"`
	// Template contains the deployment of the created virtualKubelet.
	// +optional
	Template *DeploymentTemplate `json:"template,omitempty"`
	// OffloadingPatch contains the information to target a groups of node on the remote cluster.
	OffloadingPatch *OffloadingPatch `json:"offloadingPatch,omitempty"`
	// CreateNode indicates if a node to target the remote cluster (and schedule on it) has to be created.
	CreateNode *bool `json:"createNode,omitempty"`
	// DisableNetworkCheck disables the check of the liqo networking.
	// If check is disabled, the network status will not be added to node conditions.
	DisableNetworkCheck *bool `json:"disableNetworkCheck,omitempty"`
	// KubeconfigSecretRef contains the reference to the secret containing the kubeconfig to access the remote cluster.
	KubeconfigSecretRef *corev1.LocalObjectReference `json:"kubeconfigSecretRef,omitempty"`
	// Images is the list of the images already stored in the cluster.
	Images []corev1.ContainerImage `json:"images,omitempty"`
	// ResourceQuota contains the quantity of resources assigned to the VirtualNode.
	ResourceQuota corev1.ResourceQuotaSpec `json:"resourceQuota,omitempty"`
	// Labels contains the labels to be added to the virtual node.
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations contains the annotations to be added to the virtual node.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Taints contains the taints to be added to the virtual node.
	Taints []corev1.Taint `json:"taints,omitempty"`
	// StorageClasses contains the list of the storage classes offered by the cluster.
	StorageClasses []liqov1beta1.StorageType `json:"storageClasses,omitempty"`
	// IngressClasses contains the list of the ingress classes offered by the cluster.
	IngressClasses []liqov1beta1.IngressType `json:"ingressClasses,omitempty"`
	// LoadBalancerClasses contains the list of the load balancer classes offered by the cluster.
	LoadBalancerClasses []liqov1beta1.LoadBalancerType `json:"loadBalancerClasses,omitempty"`
	// VkOptionsTemplateRef contains the namespaced reference to the VkOptionsTemplate.
	// If not set, the default template installed with Liqo will be used.
	// +optional
	VkOptionsTemplateRef *corev1.ObjectReference `json:"vkOptionsTemplateRef,omitempty"`
}

// VirtualNodeConditionType represents different conditions that a virtualNode could assume.
type VirtualNodeConditionType string

const (
	// VirtualKubeletConditionType informs users about the VirtualKubelet status.
	VirtualKubeletConditionType VirtualNodeConditionType = "VirtualKubelet"
	// NodeConditionType informs users about the Node status.
	NodeConditionType VirtualNodeConditionType = "Node"
)

// VirtualNodeConditionStatusType represents different statuses that a condition could assume.
type VirtualNodeConditionStatusType string

const (
	// NoneConditionStatusType represents the absence of a condition.
	NoneConditionStatusType VirtualNodeConditionStatusType = "None"
	// RunningConditionStatusType represents the condition is in running state.
	RunningConditionStatusType VirtualNodeConditionStatusType = "Running"
	// CreatingConditionStatusType represents the condition is in creating state.
	CreatingConditionStatusType VirtualNodeConditionStatusType = "Creating"
	// DrainingConditionStatusType represents the condition is in draining state.
	DrainingConditionStatusType VirtualNodeConditionStatusType = "Draining"
	// DeletingConditionStatusType represents the condition is in deleting state.
	DeletingConditionStatusType VirtualNodeConditionStatusType = "Deleting"
)

// VirtualNodeCondition contains some information about remote namespace status.
type VirtualNodeCondition struct {
	// Type of the VirtualNode condition.
	// +kubebuilder:validation:Enum="VirtualKubelet";"Node"
	Type VirtualNodeConditionType `json:"type"`
	// Status of the condition.
	// +kubebuilder:validation:Enum="None";"Running";"Creating";"Draining";"Deleting"
	// +kubebuilder:default="None"
	Status VirtualNodeConditionStatusType `json:"status"`
	// LastTransitionTime -> timestamp for when the condition last transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason -> Machine-readable, UpperCamelCase text indicating the reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message -> Human-readable message indicating details about the last status transition.
	Message string `json:"message,omitempty"`
}

// VirtualNodeStatus contains some information about remote namespace status.
type VirtualNodeStatus struct {
	Conditions []VirtualNodeCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=vn;vnode;virno;vno
// +kubebuilder:subresource:status
// +genclient
// +kubebuilder:printcolumn:name="ClusterID",type=string,JSONPath=`.spec.clusterID`
// +kubebuilder:printcolumn:name="Create Node",type=boolean,JSONPath=`.spec.createNode`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.status.conditions[?(@.type=="Node")].status`,priority=1
// +kubebuilder:printcolumn:name="VirtualKubelet",type=string,JSONPath=`.status.conditions[?(@.type=="VirtualKubelet")].status`,priority=1

// VirtualNode is the Schema for the VirtualNodes API.
type VirtualNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualNodeSpec   `json:"spec,omitempty"`
	Status VirtualNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VirtualNodeList contains a list of VirtualNode.
type VirtualNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualNode{}, &VirtualNodeList{})
}
