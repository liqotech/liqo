// Copyright 2019-2022 The Liqo Authors
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

package offload

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/args"
)

func TestOffloadCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Parameters Fetching")
}

var _ = Describe("Test the generate command works as expected", func() {

	type testCase struct {
		args           []string
		parameters     []string
		acceptedLabels args.StringMap
		deniedLabels   args.StringMap
		expected       v1alpha1.NamespaceOffloading
	}

	DescribeTable("A generate command is performed",
		func(tc testCase) {
			cmd := &cobra.Command{}
			cmd.SetArgs(tc.parameters)
			cmd.PersistentFlags().String(PodOffloadingStrategyFlag, string(v1alpha1.LocalAndRemotePodOffloadingStrategyType), "")
			cmd.PersistentFlags().String(NamespaceMappingStrategyFlag, string(v1alpha1.DefaultNameMappingStrategyType), "")
			Expect(cmd.Execute()).To(Succeed())
			nsOffloading := forgeNamespaceOffloading(cmd, tc.args, tc.acceptedLabels, tc.deniedLabels)
			Expect(nsOffloading.ObjectMeta).To(MatchFields(IgnoreExtras, Fields{
				"Name":      Equal(tc.expected.Name),
				"Namespace": Equal(tc.expected.Namespace),
			}))
			Expect(nsOffloading.Spec).To(MatchFields(IgnoreExtras, Fields{
				"NamespaceMappingStrategy": Equal(tc.expected.Spec.NamespaceMappingStrategy),
				"PodOffloadingStrategy":    Equal(tc.expected.Spec.PodOffloadingStrategy),
				"ClusterSelector":          Equal(tc.expected.Spec.ClusterSelector),
			}))
		},
		Entry("Offload namespace with default parameters", testCase{
			[]string{"test"},
			[]string{},
			args.StringMap{StringMap: map[string]string{}},
			args.StringMap{StringMap: map[string]string{}},
			v1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: "test",
				},
				Spec: v1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: v1alpha1.DefaultNameMappingStrategyType,
					PodOffloadingStrategy:    v1alpha1.LocalAndRemotePodOffloadingStrategyType,
					ClusterSelector: corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
						MatchExpressions: []corev1.NodeSelectorRequirement{{
							Key:      liqoconst.TypeLabel,
							Operator: corev1.NodeSelectorOpIn,
							Values:   []string{liqoconst.TypeNode},
						}}},
					}},
				},
			},
		}),
		Entry("Offload namespace with accepted/denied labels, custom pod-offloading and namespace mapping strategy",
			testCase{
				[]string{"test"},
				[]string{
					"--pod-offloading-strategy=Local",
				},
				args.StringMap{StringMap: map[string]string{
					"accepted": "true",
				}},
				args.StringMap{StringMap: map[string]string{
					"denied": "true",
				}},
				v1alpha1.NamespaceOffloading{
					ObjectMeta: metav1.ObjectMeta{
						Name:      liqoconst.DefaultNamespaceOffloadingName,
						Namespace: "test",
					},
					Spec: v1alpha1.NamespaceOffloadingSpec{
						NamespaceMappingStrategy: v1alpha1.DefaultNameMappingStrategyType,
						PodOffloadingStrategy:    v1alpha1.LocalPodOffloadingStrategyType,
						ClusterSelector: corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      liqoconst.TypeLabel,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{liqoconst.TypeNode},
										},
										{
											Key:      "accepted",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"true"},
										},
										{
											Key:      "denied",
											Operator: corev1.NodeSelectorOpNotIn,
											Values:   []string{"true"},
										},
									},
								},
							}},
					},
				},
			}),
		Entry("Offload namespace with default parameters", testCase{
			[]string{"test"},
			[]string{
				"--pod-offloading-strategy=Local",
			},
			args.StringMap{StringMap: map[string]string{
				"accepted": "true",
			}},
			args.StringMap{StringMap: map[string]string{
				"denied": "true",
			}},
			v1alpha1.NamespaceOffloading{
				ObjectMeta: metav1.ObjectMeta{
					Name:      liqoconst.DefaultNamespaceOffloadingName,
					Namespace: "test",
				},
				Spec: v1alpha1.NamespaceOffloadingSpec{
					NamespaceMappingStrategy: v1alpha1.DefaultNameMappingStrategyType,
					PodOffloadingStrategy:    v1alpha1.LocalPodOffloadingStrategyType,
					ClusterSelector: corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      liqoconst.TypeLabel,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{liqoconst.TypeNode},
									},
									{
										Key:      "accepted",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"true"},
									},
									{
										Key:      "denied",
										Operator: corev1.NodeSelectorOpNotIn,
										Values:   []string{"true"},
									},
								},
							},
						}},
				},
			},
		}),
		Entry("Offload namespace with accepted, custom pod-offloading and namespace mapping strategy",
			testCase{
				[]string{"test"},
				[]string{
					"--namespace-mapping-strategy=EnforceSameName",
					"--pod-offloading-strategy=Remote",
				},
				args.StringMap{StringMap: map[string]string{
					"test": "true",
				}},
				args.StringMap{StringMap: map[string]string{}},
				v1alpha1.NamespaceOffloading{
					ObjectMeta: metav1.ObjectMeta{
						Name:      liqoconst.DefaultNamespaceOffloadingName,
						Namespace: "test",
					},
					Spec: v1alpha1.NamespaceOffloadingSpec{
						NamespaceMappingStrategy: v1alpha1.EnforceSameNameMappingStrategyType,
						PodOffloadingStrategy:    v1alpha1.RemotePodOffloadingStrategyType,
						ClusterSelector: corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{
									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      liqoconst.TypeLabel,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{liqoconst.TypeNode},
										},
										{
											Key:      "test",
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{"true"},
										},
									},
								},
							}},
					},
				},
			}),
		Entry("Offload namespace with denied labels only",
			testCase{
				[]string{"test"},
				[]string{
					"--namespace-mapping-strategy=EnforceSameName",
					"--pod-offloading-strategy=Remote",
				},
				args.StringMap{StringMap: map[string]string{}},
				args.StringMap{StringMap: map[string]string{
					"test": "true",
				}},
				v1alpha1.NamespaceOffloading{
					ObjectMeta: metav1.ObjectMeta{
						Name:      liqoconst.DefaultNamespaceOffloadingName,
						Namespace: "test",
					},
					Spec: v1alpha1.NamespaceOffloadingSpec{
						NamespaceMappingStrategy: v1alpha1.EnforceSameNameMappingStrategyType,
						PodOffloadingStrategy:    v1alpha1.RemotePodOffloadingStrategyType,
						ClusterSelector: corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{

									MatchExpressions: []corev1.NodeSelectorRequirement{
										{
											Key:      liqoconst.TypeLabel,
											Operator: corev1.NodeSelectorOpIn,
											Values:   []string{liqoconst.TypeNode},
										},
										{
											Key:      "test",
											Operator: corev1.NodeSelectorOpNotIn,
											Values:   []string{"true"},
										},
									},
								},
							}},
					},
				},
			}),
	)
})
