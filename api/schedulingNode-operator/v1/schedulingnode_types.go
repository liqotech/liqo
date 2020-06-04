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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SchedulingNodeSpec defines the desired state of SchedulingNode
type SchedulingNodeSpec struct {
	NodeName      corev1.ResourceName                         `json:"nodeName"`
	NodeType      corev1.ResourceName                         `json:"nodeType"`
	Images        []corev1.ContainerImage                     `json:"images,omitempty"`
	LimitRange    corev1.LimitRangeSpec                       `json:"limitRange,omitempty"`
	ResourceQuota corev1.ResourceQuotaSpec                    `json:"resourceQuota,omitempty"`
	Neighbors     map[corev1.ResourceName]corev1.ResourceList `json:"neighbors,omitempty"`
	Properties    map[corev1.ResourceName]string              `json:"properties,omitempty"`
	Prices        corev1.ResourceList                         `json:"prices,omitempty"`
}

// SchedulingNodeStatus defines the observed state of SchedulingNode
type SchedulingNodeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// SchedulingNode is the Schema for the schedulingnodes API
type SchedulingNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchedulingNodeSpec   `json:"spec,omitempty"`
	Status SchedulingNodeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SchedulingNodeList contains a list of SchedulingNode
type SchedulingNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchedulingNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SchedulingNode{}, &SchedulingNodeList{})
}
