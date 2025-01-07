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

package testutils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// GetNamespaceOffloading gets the right NamespaceOffloading according to the specified strategy.
func GetNamespaceOffloading(strategy offloadingv1beta1.PodOffloadingStrategyType) offloadingv1beta1.NamespaceOffloading {
	return offloadingv1beta1.NamespaceOffloading{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "offloading",
			Namespace: "test",
		},
		Spec: offloadingv1beta1.NamespaceOffloadingSpec{
			NamespaceMappingStrategy: offloadingv1beta1.EnforceSameNameMappingStrategyType,
			PodOffloadingStrategy:    strategy,
			ClusterSelector:          GetImposedNodeSelector(""),
		},
	}
}

// GetImposedNodeSelector gets the right imposedSelector according to the specified strategy.
func GetImposedNodeSelector(strategy offloadingv1beta1.PodOffloadingStrategyType) corev1.NodeSelector {
	var nodeSelector corev1.NodeSelector
	switch {
	case strategy == offloadingv1beta1.RemotePodOffloadingStrategyType:
		nodeSelector = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"A,B"},
					},
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"C,D"},
					},
					{
						Key:      "NotProvider",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
		}}
	case strategy == offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType:
		nodeSelector = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"A,B"},
					},
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"C,D"},
					},
					{
						Key:      "NotProvider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
		}}
	default:
		// This NodeSelector is imposed by the NamespaceOffloading.
		nodeSelector = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"A,B"},
					},
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"C,D"},
					}, {
						Key:      "NotProvider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
		},
		}
	}
	return nodeSelector
}

// GetPodNodeSelector gets a generic Pod NodeSelector.
func GetPodNodeSelector() corev1.NodeSelector {
	return corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
		{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      "storage",
					Operator: corev1.NodeSelectorOpExists,
				}},
		},
		{
			MatchExpressions: []corev1.NodeSelectorRequirement{
				{
					Key:      "provider",
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{"AWS"},
				}},
		},
	},
	}
}

// GetMergedNodeSelector gets the right mergedNodeSelector according to the specified strategy.
func GetMergedNodeSelector(strategy offloadingv1beta1.PodOffloadingStrategyType) corev1.NodeSelector {
	var mergedNodeSelector corev1.NodeSelector
	switch {
	case strategy == offloadingv1beta1.RemotePodOffloadingStrategyType:
		mergedNodeSelector = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "storage",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"A,B"},
					},
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "storage",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"C,D"},
					},
					{
						Key:      "NotProvider",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"AWS"},
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"A,B"},
					},
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"AWS"},
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"C,D"},
					},
					{
						Key:      "NotProvider",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
		}}
	case strategy == offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType:
		mergedNodeSelector = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "storage",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"A,B"},
					},
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "storage",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"C,D"},
					},
					{
						Key:      "NotProvider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "storage",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"AWS"},
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"A,B"},
					},
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"AWS"},
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"C,D"},
					},
					{
						Key:      "NotProvider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"AWS"},
					},
					{
						Key:      liqoconst.TypeLabel,
						Operator: corev1.NodeSelectorOpNotIn,
						Values:   []string{liqoconst.TypeNode},
					},
				},
			},
		}}
	default:
		mergedNodeSelector = corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "storage",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"A,B"},
					},
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "storage",
						Operator: corev1.NodeSelectorOpExists,
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"C,D"},
					},
					{
						Key:      "NotProvider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"AWS"},
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"A,B"},
					},
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "provider",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"AWS"},
					},
					{
						Key:      "region",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"C,D"},
					},
					{
						Key:      "NotProvider",
						Operator: corev1.NodeSelectorOpExists,
					},
				},
			},
		}}
	}
	return mergedNodeSelector
}
