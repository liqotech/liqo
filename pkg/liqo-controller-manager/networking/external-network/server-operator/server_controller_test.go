// Copyright 2019-2026 The Liqo Authors
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

package serveroperator

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServerOperator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Operator Suite")
}

var _ = Describe("mergeServiceAnnotations", func() {
	type testCase struct {
		spec        interface{}
		annotations map[string]string
		expected    interface{}
	}

	DescribeTable("merging service annotations",
		func(tc testCase) {
			mergeServiceAnnotations(tc.spec, tc.annotations)
			Expect(tc.spec).To(Equal(tc.expected))
		},

		Entry("nil annotations does nothing", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
			annotations: nil,
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
		}),

		Entry("empty annotations does nothing", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
			annotations: map[string]string{},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
		}),

		Entry("adds annotations to existing service metadata", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
			annotations: map[string]string{
				"new-key": "new-value",
			},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"existing": "value",
							"new-key":  "new-value",
						},
					},
				},
			},
		}),

		Entry("overrides existing annotations", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"key": "old-value",
						},
					},
				},
			},
			annotations: map[string]string{
				"key": "new-value",
			},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"key": "new-value",
						},
					},
				},
			},
		}),

		Entry("creates service map when missing", testCase{
			spec: map[string]interface{}{
				"deployment": map[string]interface{}{},
			},
			annotations: map[string]string{
				"key": "value",
			},
			expected: map[string]interface{}{
				"deployment": map[string]interface{}{},
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"key": "value",
						},
					},
				},
			},
		}),

		Entry("creates metadata map when missing", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"spec": map[string]interface{}{},
				},
			},
			annotations: map[string]string{
				"key": "value",
			},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"spec": map[string]interface{}{},
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"key": "value",
						},
					},
				},
			},
		}),

		Entry("creates annotations map when missing", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "test",
						},
					},
				},
			},
			annotations: map[string]string{
				"key": "value",
			},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app": "test",
						},
						"annotations": map[string]interface{}{
							"key": "value",
						},
					},
				},
			},
		}),

		Entry("non-map spec is a no-op", testCase{
			spec:        "not a map",
			annotations: map[string]string{"key": "value"},
			expected:    "not a map",
		}),
	)
})
