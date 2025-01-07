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
//

package utils_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/utils"
)

type sampleDataStructure struct {
	Number         int
	Value          string
	OptionalString *string
	OptionalNumber *int
}

type nestedSampleDataStructure struct {
	Number         int
	Value          string
	OptionalString *string
	OptionalNumber *int
	Nested         sampleDataStructure
}

type testDataStructure struct {
	Name      string
	Namespace string
	Spec      nestedSampleDataStructure
}

type templateTestCase struct {
	template    any
	expectedRes any
	data        testDataStructure
}

var optionalNumber = 10
var optionalString = "Mr. Jack!"
var nestedOptionalString = "optionalVal"

var _ = DescribeTable("Templating tests", func(testCase templateTestCase) {
	res, err := utils.RenderTemplate(testCase.template, testCase.data, false)

	Expect(err).NotTo(HaveOccurred())
	Expect(testCase.expectedRes).To(Equal(res), "Unexpected result returned")
},
	Entry("Simple case", templateTestCase{
		template: map[string]any{
			"Name":      "{{ .Name }}",
			"Namespace": "{{ .Namespace }}",
		},
		expectedRes: map[string]any{
			"Name":      "hello",
			"Namespace": "world!",
		},
		data: testDataStructure{
			Name:      "hello",
			Namespace: "world!",
		},
	}),
	Entry("Labels and annotations should force string", templateTestCase{
		template: map[string]any{
			"labels": map[string]any{
				"hello": "{{ .Namespace }}",
				"test":  4,
			},
			"annotations": map[string]any{
				"hello": "{{ .Name }}",
				"test":  5,
			},
		},
		expectedRes: map[string]any{
			"labels": map[string]any{
				"hello": "world!",
				"test":  "4",
			},
			"annotations": map[string]any{
				"hello": "hello",
				"test":  "5",
			},
		},
		data: testDataStructure{
			Name:      "hello",
			Namespace: "world!",
		},
	}),
	Entry("Nested variables", templateTestCase{
		template: map[string]any{
			"Name": "{{ .Name }}",
			"Nested": map[string]any{
				"NestedVal": map[string]any{
					"Value":  "{{ .Spec.Nested.Value }}",
					"Number": "{{ .Spec.Nested.Number }}",
				},
				"ListVal": []any{
					map[string]any{
						"Another": "{{ .Spec.Value }}",
					},
					map[string]any{
						"Number": "{{ .Spec.Number }}",
					},
				},
			},
		},
		expectedRes: map[string]any{
			"Name": "hello",
			"Nested": map[string]any{
				"NestedVal": map[string]any{
					"Value":  "world!",
					"Number": 1924,
				},
				"ListVal": []any{
					map[string]any{
						"Another": "value",
					},
					map[string]any{
						"Number": 10,
					},
				},
			},
		},
		data: testDataStructure{
			Name: "hello",
			Spec: nestedSampleDataStructure{
				Value:  "value",
				Number: 10,
				Nested: sampleDataStructure{
					Value:  "world!",
					Number: 1924,
				},
			},
		},
	}),
	Entry("Optional fields", templateTestCase{
		template: map[string]any{
			"Name":         "{{ .Name }}",
			"?NotOptional": "This should be kept as is",
			"Nested": map[string]any{
				"NestedVal": map[string]any{
					"Value":           "{{ .Spec.Nested.Value }}",
					"?Optional":       "Some text plus variable {{ .Spec.Nested.OptionalString }}",
					"?OptionalNumber": "{{ .Spec.Nested.OptionalNumber }}",
				},
				"ListVal": []any{
					map[string]any{
						"Another": "{{ .Spec.Value }}",
						"?Hey":    "{{ .Spec.OptionalString }}",
					},
					map[string]any{
						"number":                        "{{ .Spec.Number }}",
						"?anotherNumber":                "{{ .Spec.OptionalNumber }}",
						"?optionalDifferentThanPointer": "{{ .Spec.Value }}",
					},
				},
			},
		},
		expectedRes: map[string]any{
			"Name":         "hello",
			"?NotOptional": "This should be kept as is",
			"Nested": map[string]any{
				"NestedVal": map[string]any{
					"Value":          "world!",
					"Optional":       fmt.Sprintf("Some text plus variable %s", nestedOptionalString),
					"OptionalNumber": optionalNumber,
				},
				"ListVal": []any{
					map[string]any{
						"Another": "value",
						"Hey":     optionalString,
					},
					map[string]any{
						"number":                       42,
						"optionalDifferentThanPointer": "value",
					},
				},
			},
		},
		data: testDataStructure{
			Name: "hello",
			Spec: nestedSampleDataStructure{
				Value:          "value",
				Number:         42,
				OptionalString: &optionalString,
				Nested: sampleDataStructure{
					Value:          "world!",
					Number:         1924,
					OptionalNumber: &optionalNumber,
					OptionalString: &nestedOptionalString,
				},
			},
		},
	}),
)

func TestLocal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "External network utils test suite")
}
