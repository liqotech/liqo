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

package offload_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/liqotech/liqo/pkg/liqoctl/offload"
)

var _ = Describe("Handler tests", func() {
	type ClusterSelectorCase struct {
		Selectors  []string
		Expected   [][]metav1.LabelSelectorRequirement
		ErrMatcher types.GomegaMatcher
	}

	DescribeTable("cluster selectors parsing",
		func(c ClusterSelectorCase) {
			var opts offload.Options
			Expect(opts.ParseClusterSelectors(c.Selectors)).To(c.ErrMatcher)
			Expect(opts.ClusterSelector).To(ContainElements(c.Expected))
		},
		Entry("equality selector", ClusterSelectorCase{
			Selectors:  []string{"key=value"},
			Expected:   [][]metav1.LabelSelectorRequirement{{{Key: "key", Operator: metav1.LabelSelectorOpIn, Values: []string{"value"}}}},
			ErrMatcher: Not(HaveOccurred()),
		}),
		Entry("multiple selectors in logical AND", ClusterSelectorCase{
			Selectors: []string{"key=value,!staging"},
			Expected: [][]metav1.LabelSelectorRequirement{{
				{Key: "staging", Operator: metav1.LabelSelectorOpDoesNotExist, Values: []string{}},
				{Key: "key", Operator: metav1.LabelSelectorOpIn, Values: []string{"value"}},
			}},
			ErrMatcher: Not(HaveOccurred()),
		}),
		Entry("multiple selectors in logical OR", ClusterSelectorCase{
			Selectors: []string{"key=value", "!staging"},
			Expected: [][]metav1.LabelSelectorRequirement{
				{{Key: "key", Operator: metav1.LabelSelectorOpIn, Values: []string{"value"}}},
				{{Key: "staging", Operator: metav1.LabelSelectorOpDoesNotExist, Values: []string{}}},
			},
			ErrMatcher: Not(HaveOccurred()),
		}),
	)
})
