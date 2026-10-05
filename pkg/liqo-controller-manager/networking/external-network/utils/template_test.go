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
//

package utils_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
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

// portsTestData is the data used to render the +ports directive.
type portsTestData struct {
	Ports     []int32
	NodePorts []int32
	Bad       string
}

type expandPortsTestCase struct {
	template map[string]any
	data     portsTestData
	want     map[string]any
}

type expandPortsErrorTestCase struct {
	template map[string]any
	data     portsTestData
	errMsg   string
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

// svcPort forges the expected Service port. An empty name or a zero nodePort leave the field unset.
func svcPort(port int64, name string, nodePort int64) map[string]any {
	m := map[string]any{
		"port":       port,
		"targetPort": port,
		"protocol":   "UDP",
	}
	if name != "" {
		m["name"] = name
	}
	if nodePort != 0 {
		m["nodePort"] = nodePort
	}
	return m
}

var _ = Describe("RenderTemplate with the +ports directive", func() {

	DescribeTable("should expand the directive", func(tc expandPortsTestCase) {
		res, err := utils.RenderTemplate(tc.template, tc.data, false)

		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(Equal(tc.want))
	},
		Entry("Multiple ports get sequential names", expandPortsTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Ports }}"}},
			data:     portsTestData{Ports: []int32{51840, 51841, 51842}},
			want: map[string]any{"ports": []any{
				svcPort(51840, "liqo-tunnel-0", 0),
				svcPort(51841, "liqo-tunnel-1", 0),
				svcPort(51842, "liqo-tunnel-2", 0),
			}},
		}),
		Entry("A single port has no name", expandPortsTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Ports }}"}},
			data:     portsTestData{Ports: []int32{51840}},
			want:     map[string]any{"ports": []any{svcPort(51840, "", 0)}},
		}),
		Entry("Custom name prefix", expandPortsTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Ports }}", "namePrefix": "wg"}},
			data:     portsTestData{Ports: []int32{51840, 51841}},
			want: map[string]any{"ports": []any{
				svcPort(51840, "wg-0", 0),
				svcPort(51841, "wg-1", 0),
			}},
		}),
		Entry("Node ports are assigned by position", expandPortsTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Ports }}", "nodePorts": "{{ .NodePorts }}"}},
			data:     portsTestData{Ports: []int32{51840, 51841}, NodePorts: []int32{30001, 30002}},
			want: map[string]any{"ports": []any{
				svcPort(51840, "liqo-tunnel-0", 30001),
				svcPort(51841, "liqo-tunnel-1", 30002),
			}},
		}),
		Entry("A single port with a node port", expandPortsTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Ports }}", "nodePorts": "{{ .NodePorts }}"}},
			data:     portsTestData{Ports: []int32{51840}, NodePorts: []int32{30001}},
			want:     map[string]any{"ports": []any{svcPort(51840, "", 30001)}},
		}),
		Entry("Fewer node ports than ports: the last ports have no node port", expandPortsTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Ports }}", "nodePorts": "{{ .NodePorts }}"}},
			data:     portsTestData{Ports: []int32{51840, 51841, 51842}, NodePorts: []int32{30001}},
			want: map[string]any{"ports": []any{
				svcPort(51840, "liqo-tunnel-0", 30001),
				svcPort(51841, "liqo-tunnel-1", 0),
				svcPort(51842, "liqo-tunnel-2", 0),
			}},
		}),
		Entry("More node ports than ports: the extra ones are ignored", expandPortsTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Ports }}", "nodePorts": "{{ .NodePorts }}"}},
			data:     portsTestData{Ports: []int32{51840}, NodePorts: []int32{30001, 30002}},
			want:     map[string]any{"ports": []any{svcPort(51840, "", 30001)}},
		}),
		Entry("No ports produces an empty list", expandPortsTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Ports }}"}},
			data:     portsTestData{},
			want:     map[string]any{"ports": []any{}},
		}),
		Entry("A template without the directive is left untouched", expandPortsTestCase{
			template: map[string]any{
				"type":  "ClusterIP",
				"ports": []any{map[string]any{"port": 80}},
			},
			want: map[string]any{
				"type":  "ClusterIP",
				"ports": []any{map[string]any{"port": 80}},
			},
		}),
		Entry("The directive takes precedence over a regular ports field", expandPortsTestCase{
			template: map[string]any{
				"+ports": map[string]any{"Ports": "{{ .Ports }}"},
				"ports":  []any{map[string]any{"port": 80}},
			},
			data: portsTestData{Ports: []int32{51840}},
			want: map[string]any{"ports": []any{svcPort(51840, "", 0)}},
		}),
	)

	It("should expand the directive when nested in maps and lists, keeping the sibling fields", func() {
		tmpl := map[string]any{
			"spec": map[string]any{
				"type":   "NodePort",
				"+ports": map[string]any{"Ports": "{{ .Ports }}"},
			},
			"items": []any{
				map[string]any{
					"name":   "first",
					"+ports": map[string]any{"Ports": "{{ .Ports }}"},
				},
			},
		}

		res, err := utils.RenderTemplate(tmpl, portsTestData{Ports: []int32{51840, 51841}}, false)

		Expect(err).NotTo(HaveOccurred())
		Expect(res).To(Equal(map[string]any{
			"spec": map[string]any{
				"type":  "NodePort",
				"ports": []any{svcPort(51840, "liqo-tunnel-0", 0), svcPort(51841, "liqo-tunnel-1", 0)},
			},
			"items": []any{
				map[string]any{
					"name":  "first",
					"ports": []any{svcPort(51840, "liqo-tunnel-0", 0), svcPort(51841, "liqo-tunnel-1", 0)},
				},
			},
		}))
	})

	DescribeTable("should fail with an invalid directive", func(tc expandPortsErrorTestCase) {
		_, err := utils.RenderTemplate(tc.template, tc.data, false)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(tc.errMsg))
	},
		Entry("the directive is not a map", expandPortsErrorTestCase{
			template: map[string]any{"+ports": "{{ .Ports }}"},
			data:     portsTestData{Ports: []int32{51840}},
			errMsg:   "+ports must be a map",
		}),
		Entry("the Ports field is missing", expandPortsErrorTestCase{
			template: map[string]any{"+ports": map[string]any{"nodePorts": "{{ .NodePorts }}"}},
			data:     portsTestData{NodePorts: []int32{30001}},
			errMsg:   "missing 'Ports' field",
		}),
		Entry("a port is not a number", expandPortsErrorTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Bad }}"}},
			data:     portsTestData{Bad: "[abc]"},
			errMsg:   "invalid port value abc",
		}),
		Entry("a node port is not a number", expandPortsErrorTestCase{
			template: map[string]any{"+ports": map[string]any{"Ports": "{{ .Ports }}", "nodePorts": "{{ .Bad }}"}},
			data:     portsTestData{Ports: []int32{51840}, Bad: "[abc]"},
			errMsg:   "invalid nodePort value abc",
		}),
	)
})

// serviceSpecTemplate mirrors the (Helm-rendered) service spec of the gateway server template.
// It is a function because RenderTemplate mutates the map in place.
func serviceSpecTemplate() map[string]any {
	return map[string]any{
		"type":            "{{ .Spec.Endpoint.ServiceType }}",
		"?loadBalancerIP": "{{ .Spec.Endpoint.LoadBalancerIP }}",
		"+ports": map[string]any{
			"Ports": "{{ if .Spec.Endpoint.Ports }}{{ .Spec.Endpoint.Ports }}{{ else }}[{{ .Spec.Endpoint.Port }}]{{ end }}",
			"nodePorts": "{{ if .Spec.Endpoint.NodePorts }}{{ .Spec.Endpoint.NodePorts }}" +
				"{{ else if .Spec.Endpoint.NodePort }}[{{ .Spec.Endpoint.NodePort }}]{{ end }}",
		},
	}
}

var _ = Describe("RenderTemplate with the gateway server service template", func() {
	render := func(ep networkingv1beta1.Endpoint) map[string]any {
		gw := networkingv1beta1.GatewayServer{Spec: networkingv1beta1.GatewayServerSpec{Endpoint: ep}}
		res, err := utils.RenderTemplate(serviceSpecTemplate(), gw, false)
		Expect(err).NotTo(HaveOccurred())
		return res.(map[string]any)
	}

	It("should render a LoadBalancer with the IP and multiple ports", func() {
		res := render(networkingv1beta1.Endpoint{
			ServiceType:    corev1.ServiceTypeLoadBalancer,
			LoadBalancerIP: ptr.To("203.0.113.5"),
			Ports:          []int32{51840, 51841},
		})

		Expect(res).To(Equal(map[string]any{
			"type":           "LoadBalancer",
			"loadBalancerIP": "203.0.113.5",
			"ports": []any{
				svcPort(51840, "liqo-tunnel-0", 0),
				svcPort(51841, "liqo-tunnel-1", 0),
			},
		}))
	})

	It("should omit loadBalancerIP when it is not set", func() {
		res := render(networkingv1beta1.Endpoint{
			ServiceType: corev1.ServiceTypeLoadBalancer,
			Ports:       []int32{51840, 51841},
		})

		Expect(res).NotTo(HaveKey("loadBalancerIP"))
		Expect(res).To(HaveKeyWithValue("type", "LoadBalancer"))
	})

	It("should render a LoadBalancer with the legacy single port", func() {
		res := render(networkingv1beta1.Endpoint{
			ServiceType: corev1.ServiceTypeLoadBalancer,
			Port:        51840, //nolint:staticcheck // Port is intentionally used for backward compatibility.
		})

		Expect(res).To(HaveKeyWithValue("ports", []any{svcPort(51840, "", 0)}))
	})

	It("should render a NodePort with the node ports", func() {
		res := render(networkingv1beta1.Endpoint{
			ServiceType: corev1.ServiceTypeNodePort,
			Ports:       []int32{51840, 51841},
			NodePorts:   []int32{30001, 30002},
		})

		Expect(res).To(HaveKeyWithValue("ports", []any{
			svcPort(51840, "liqo-tunnel-0", 30001),
			svcPort(51841, "liqo-tunnel-1", 30002),
		}))
	})
})

func TestLocal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "External network utils test suite")
}
