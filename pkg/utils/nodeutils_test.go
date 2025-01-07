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

package utils

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

var _ = Describe("NodeUtils", func() {

	Context("isVirtualNode", func() {

		type isVirtualNodeTestcase struct {
			node          *v1.Node
			expectedValue types.GomegaMatcher
		}

		DescribeTable("isVirtualNode table",
			func(c isVirtualNodeTestcase) {
				Expect(IsVirtualNode(c.node)).To(c.expectedValue)
			},

			Entry("no label", isVirtualNodeTestcase{
				node:          &v1.Node{},
				expectedValue: BeFalse(),
			}),

			Entry("other labels", isVirtualNodeTestcase{
				node: &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"label1": "value1",
						},
					},
				},
				expectedValue: BeFalse(),
			}),

			Entry("invalid label", isVirtualNodeTestcase{
				node: &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							liqoconst.TypeLabel: "value1",
						},
					},
				},
				expectedValue: BeFalse(),
			}),

			Entry("valid label", isVirtualNodeTestcase{
				node: &v1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							liqoconst.TypeLabel: liqoconst.TypeNode,
						},
					},
				},
				expectedValue: BeTrue(),
			}),
		)

	})

	Context("mergeNodeSelector", func() {

		var (
			getExpression = func(n int) v1.NodeSelectorRequirement {
				return v1.NodeSelectorRequirement{
					Key:      fmt.Sprintf("label%v", n),
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{fmt.Sprintf("val%v", n)},
				}
			}
		)

		type mergeNodeSelectorTestcase struct {
			nodeSelector1    *v1.NodeSelector
			nodeSelector2    *v1.NodeSelector
			expectedSelector v1.NodeSelector
		}

		DescribeTable("mergeNodeSelector table",
			func(c mergeNodeSelectorTestcase) {
				Expect(MergeNodeSelector(c.nodeSelector1, c.nodeSelector2)).To(Equal(c.expectedSelector))
			},

			Entry("both selectors with one term and one expression", mergeNodeSelectorTestcase{
				nodeSelector1: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
							},
						},
					},
				},
				nodeSelector2: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(2),
							},
						},
					},
				},
				expectedSelector: v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
								getExpression(2),
							},
						},
					},
				},
			}),

			Entry("first selector with one term and one expression, the other with multiple", mergeNodeSelectorTestcase{
				nodeSelector1: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
							},
						},
					},
				},
				nodeSelector2: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(2),
								getExpression(3),
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(4),
								getExpression(5),
							},
						},
					},
				},
				expectedSelector: v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
								getExpression(2),
								getExpression(3),
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
								getExpression(4),
								getExpression(5),
							},
						},
					},
				},
			}),

			Entry("second selector with one term and one expression, the other with multiple", mergeNodeSelectorTestcase{
				nodeSelector1: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
								getExpression(2),
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(3),
								getExpression(4),
							},
						},
					},
				},
				nodeSelector2: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(5),
							},
						},
					},
				},
				expectedSelector: v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
								getExpression(2),
								getExpression(5),
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(3),
								getExpression(4),
								getExpression(5),
							},
						},
					},
				},
			}),

			Entry("both selectors with multiple terms and expressions", mergeNodeSelectorTestcase{
				nodeSelector1: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
								getExpression(2),
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(3),
								getExpression(4),
							},
						},
					},
				},
				nodeSelector2: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(5),
								getExpression(6),
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(7),
								getExpression(8),
							},
						},
					},
				},
				expectedSelector: v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
								getExpression(2),
								getExpression(5),
								getExpression(6),
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(1),
								getExpression(2),
								getExpression(7),
								getExpression(8),
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(3),
								getExpression(4),
								getExpression(5),
								getExpression(6),
							},
						},
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								getExpression(3),
								getExpression(4),
								getExpression(7),
								getExpression(8),
							},
						},
					},
				},
			}),
		)

	})

})
