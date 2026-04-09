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

var _ = Describe("mergeServiceMetadataField", func() {
	type testCase struct {
		spec     interface{}
		values   map[string]string
		expected interface{}
	}

	const field = "annotations"

	DescribeTable("merging service metadata field",
		func(tc testCase) {
			mergeServiceMetadataField(tc.spec, field, tc.values)
			Expect(tc.spec).To(Equal(tc.expected))
		},

		Entry("nil values does nothing", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
			values: nil,
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
		}),

		Entry("empty values does nothing", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
			values: map[string]string{},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
		}),

		Entry("adds values to existing service metadata", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
							"existing": "value",
						},
					},
				},
			},
			values: map[string]string{
				"new-key": "new-value",
			},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
							"existing": "value",
							"new-key":  "new-value",
						},
					},
				},
			},
		}),

		Entry("overrides existing values", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
							"key": "old-value",
						},
					},
				},
			},
			values: map[string]string{
				"key": "new-value",
			},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
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
			values: map[string]string{
				"key": "value",
			},
			expected: map[string]interface{}{
				"deployment": map[string]interface{}{},
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
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
			values: map[string]string{
				"key": "value",
			},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"spec": map[string]interface{}{},
					"metadata": map[string]interface{}{
						field: map[string]interface{}{
							"key": "value",
						},
					},
				},
			},
		}),

		Entry("creates field map when missing", testCase{
			spec: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"other": map[string]interface{}{
							"app": "test",
						},
					},
				},
			},
			values: map[string]string{
				"key": "value",
			},
			expected: map[string]interface{}{
				"service": map[string]interface{}{
					"metadata": map[string]interface{}{
						"other": map[string]interface{}{
							"app": "test",
						},
						field: map[string]interface{}{
							"key": "value",
						},
					},
				},
			},
		}),

		Entry("non-map spec is a no-op", testCase{
			spec:     "not a map",
			values:   map[string]string{"key": "value"},
			expected: "not a map",
		}),
	)
})
