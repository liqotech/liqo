// Copyright 2019-2021 The Liqo Authors
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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeploymentTemplateSpec struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the deployment.
	Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
}

type GeneratedDeploymentStatus struct {
	DeploymentLastCondition appsv1.DeploymentCondition `json:"deploymentLastCondition,omitempty"`

	DeploymentLabels map[string]string `json:"deploymentLabels,omitempty"`
}

type LiqoDeploymentSpec struct {

	// Template describes the deployments that will be created.
	// +kubebuilder:pruning:PreserveUnknownFields
	Template DeploymentTemplateSpec `json:"template"`

	// GroupByLabels
	GroupByLabels []string `json:"groupByLabels,omitempty"`

	// ClusterFilter allows users to select the clusters they want to exclude from groupBy operation.
	ClusterFilter corev1.NodeSelector `json:"clusterFilter,omitempty"`
}

// NamespaceOffloadingStatus defines the observed state of NamespaceOffloading.
type LiqoDeploymentStatus struct {
	// CurrentDeployment this map represents the current deployments and their actual conditions
	// the key of this map is the same of the DesiredMapping map.
	CurrentDeployment map[string]GeneratedDeploymentStatus `json:"currentDeployment,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName="ldp"

type LiqoDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LiqoDeploymentSpec   `json:"spec"`
	Status LiqoDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LiqoDeploymentList contains a list of LiqoDeployment.
type LiqoDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LiqoDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LiqoDeployment{}, &LiqoDeploymentList{})
}
