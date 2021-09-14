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

package testutils

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// Consts used by unit tests of the liqo-deployment-controller.
const (

	// Deployment label.

	labelKey   = "app"
	labelValue = "nginx"

	// Containers image.

	containerName  = "nginx"
	containerImage = "nginx:1.14.2"

	// Different types of NodeSelector.

	EmptySelector   = 0
	NoProviderC     = 1
	NoRegionB       = 2
	WrongSelector   = 3
	MergedSelector  = 4
	MockSelector    = 5
	DefaultSelector = 6

	// Different types of podAntiAffinity.

	EmptyAffinity = 0
	MockAffinity  = 1

	// Different GenerationLabels for a LiqoDeployment resource.

	EmptyGenerationLabels             = 0
	ProviderGenerationLabels          = 1
	RegionGenerationLabels            = 2
	RegionAndProviderGenerationLabels = 3

	// Combinations of labels for different replication granularities and cluster selections.

	ProviderSelectionWithoutSelector          = 0
	ProviderSelectionWithSelector             = 1
	RegionAndProviderSelectionWithoutSelector = 2
	RegionAndProviderSelectionWithSelector    = 3

	// Label exposed by virtual nodes.

	ProviderLabel = "liqo.io/provider"
	ProviderA     = "A"
	ProviderB     = "B"
	ProviderC     = "C"

	RegionLabel = "liqo.io/region"
	RegionA     = "A"
	RegionB     = "B"
	RegionC     = "C"

	defaultLabel = "kubernetes.io/hostname"

	// Separators used to build combinations of labels.

	LabelSeparator    = "&&"
	KeyValueSeparator = "="
)

// NOTE
// "Create functions" create the object in the cluster, recalling the corresponding "Get functions" if present.
// "Get functions" return a pointer to a new object to be created.

// CreateLiqoDeployment creates a new LiqoDeployment in the cluster. It is possible to choose GenerationLabels,
// SelectedClusters fields and there is also the possibility to set some affinity on Pod Template to test it.
func CreateLiqoDeployment(ctx context.Context, cl client.Client, selectedClusters corev1.NodeSelector,
	generationLabels []string, name, namespace string, podAffinity bool) (*offv1alpha1.LiqoDeployment, error) {
	liqoDeployment := GetLiqoDeployment(selectedClusters, generationLabels, name, namespace, podAffinity)
	err := cl.Create(ctx, liqoDeployment)
	return liqoDeployment, err
}

// CreateNewDeployment creates a new deployment in the cluster. There is the possibility to choose the deployment labels.
func CreateNewDeployment(ctx context.Context, cl client.Client, generationLabels map[string]string, namespace,
	name string) (*appsv1.Deployment, error) {
	deploymentLabels := map[string]string{}

	for k, v := range generationLabels {
		deploymentLabels[k] = v
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    deploymentLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelKey: labelValue,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelKey: labelValue,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  containerName,
						Image: containerImage,
						Ports: []corev1.ContainerPort{
							{
								ContainerPort: 80,
							},
						},
					},
					},
				},
			},
		},
	}
	err := cl.Create(ctx, deployment)
	return deployment, err
}

// GetLiqoDeployment retrieves just a LiqoDeployment resource. It is possible to choose GenerationLabels,
// SelectedClusters fields and there is also the possibility to set some affinity on Pod Template to test it.
func GetLiqoDeployment(selectedClusters corev1.NodeSelector,
	generationLabels []string, name, namespace string, podAffinity bool) *offv1alpha1.LiqoDeployment {
	nodeSelector := GetNodeSelector(EmptySelector)
	podAntiAffinity := GetPodAntiAffinity(EmptyAffinity)

	if podAffinity {
		nodeSelector = GetNodeSelector(NoProviderC)
		podAntiAffinity = GetPodAntiAffinity(MockAffinity)
	}

	liqoDeployment := &offv1alpha1.LiqoDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: offv1alpha1.LiqoDeploymentSpec{
			Template: offv1alpha1.DeploymentTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelKey: labelValue,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: pointer.Int32(2),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							labelKey: labelValue,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								labelKey: labelValue,
							},
						},
						Spec: corev1.PodSpec{
							Affinity: &corev1.Affinity{
								NodeAffinity: &corev1.NodeAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: nodeSelector.DeepCopy(),
								},

								PodAntiAffinity: &corev1.PodAntiAffinity{
									RequiredDuringSchedulingIgnoredDuringExecution: podAntiAffinity,
								},
							},
							Containers: []corev1.Container{{
								Name:  containerName,
								Image: containerImage,
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 80,
									},
								},
							},
							},
						},
					},
				},
			},
			GenerationLabels: generationLabels,
			SelectedClusters: selectedClusters,
		},
	}
	return liqoDeployment
}

// GetPodAntiAffinity retrieves a mock PodAntiAffinity or an empty one.
func GetPodAntiAffinity(cmd int) []corev1.PodAffinityTerm {
	topologyKey := "kubernetes.io/hostname"
	var antiAffinityTerms []corev1.PodAffinityTerm
	switch cmd {
	case EmptyAffinity:
		antiAffinityTerms = []corev1.PodAffinityTerm{}
	case MockAffinity:
		antiAffinityTerms = []corev1.PodAffinityTerm{{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelKey: labelValue,
				},
			},
			TopologyKey: topologyKey,
		},
		}
	}
	return antiAffinityTerms
}

// GetNamespaceOffloading retrieves a NamespaceOffloading resource.
// There is the possibility to choose the ClusterSelector of the resource.
func GetNamespaceOffloading(ns corev1.NodeSelector, namespace string) *offv1alpha1.NamespaceOffloading {
	return &offv1alpha1.NamespaceOffloading{
		ObjectMeta: metav1.ObjectMeta{
			Name:      liqoconst.DefaultNamespaceOffloadingName,
			Namespace: namespace,
		},
		Spec: offv1alpha1.NamespaceOffloadingSpec{
			NamespaceMappingStrategy: offv1alpha1.EnforceSameNameMappingStrategyType,
			PodOffloadingStrategy:    offv1alpha1.LocalAndRemotePodOffloadingStrategyType,
			ClusterSelector:          ns,
		},
	}
}

// GetAvailableCombinations returns possible combinations for different replication granularity and cluster selections.
func GetAvailableCombinations(cmd int) map[string]struct{} {
	selectionMap := map[string]struct{}{}
	switch cmd {
	case ProviderSelectionWithoutSelector:
		selectionMap[fmt.Sprintf("%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderA)] = struct{}{}
		selectionMap[fmt.Sprintf("%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderB)] = struct{}{}
		selectionMap[fmt.Sprintf("%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderC)] = struct{}{}
	case ProviderSelectionWithSelector:
		selectionMap[fmt.Sprintf("%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderA)] = struct{}{}
		selectionMap[fmt.Sprintf("%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderB)] = struct{}{}
	case RegionAndProviderSelectionWithoutSelector:
		selectionMap[fmt.Sprintf("%s%s%s%s%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderA,
			LabelSeparator, RegionLabel, KeyValueSeparator, RegionA)] = struct{}{}
		selectionMap[fmt.Sprintf("%s%s%s%s%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderB,
			LabelSeparator, RegionLabel, KeyValueSeparator, RegionB)] = struct{}{}
		selectionMap[fmt.Sprintf("%s%s%s%s%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderC,
			LabelSeparator, RegionLabel, KeyValueSeparator, RegionC)] = struct{}{}
	case RegionAndProviderSelectionWithSelector:
		selectionMap[fmt.Sprintf("%s%s%s%s%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderA,
			LabelSeparator, RegionLabel, KeyValueSeparator, RegionA)] = struct{}{}
		selectionMap[fmt.Sprintf("%s%s%s%s%s%s%s%s", LabelSeparator, ProviderLabel, KeyValueSeparator, ProviderB,
			LabelSeparator, RegionLabel, KeyValueSeparator, RegionB)] = struct{}{}
	}
	return selectionMap
}

// GetNodeSelector retrieves different NodeSelectors for LiqoDeployment or NamespaceOffloading resources.
func GetNodeSelector(cmd int) corev1.NodeSelector {
	var ns corev1.NodeSelector
	switch cmd {
	case EmptySelector:
		ns = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}}
	case NoProviderC:
		ns = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      ProviderLabel,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{ProviderC},
					},
				},
			},
		}}
	case NoRegionB:
		ns = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      RegionLabel,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{RegionB},
					},
				},
			},
		}}
	case WrongSelector:
		// This selector is syntactically wrong. The operator "Exists" does not have any value.
		ns = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      ProviderLabel,
						Operator: corev1.NodeSelectorOpExists,
						Values:   []string{ProviderC},
					},
				},
			},
		}}
	case MergedSelector:
		ns = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      ProviderLabel,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{ProviderC},
					},
					{
						Key:      RegionLabel,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{RegionB},
					},
				},
			},
		}}
	case MockSelector:
		ns = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      ProviderLabel,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{ProviderA},
					},
					{
						Key:      ProviderLabel,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{ProviderC},
					},
				},
			},
		}}
	case DefaultSelector:
		ns = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
		}}
	default:
		ns = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{}}
	}
	return ns
}

// GetGenerationLabels retrieves different GenerationLabels for a LiqoDeployment resource.
func GetGenerationLabels(cmd int) []string {
	var labels []string
	switch cmd {
	case EmptyGenerationLabels:
		labels = []string{}
	case ProviderGenerationLabels:
		labels = []string{ProviderLabel}
	case RegionGenerationLabels:
		labels = []string{RegionLabel}
	case RegionAndProviderGenerationLabels:
		labels = []string{ProviderLabel, RegionLabel}
	}
	return labels
}

// GetVirtualNode returns a virtual node with specific region and provider labels.
func GetVirtualNode(name, remoteClusterID, regionLabel, providerLabel string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				liqoconst.RemoteClusterID: remoteClusterID,
			},
			Labels: map[string]string{
				liqoconst.TypeLabel: liqoconst.TypeNode,
				RegionLabel:         regionLabel,
				ProviderLabel:       providerLabel,
				defaultLabel:        name,
			},
		},
	}
}

// GetNamespace returns namespaces with specific names.
func GetNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
