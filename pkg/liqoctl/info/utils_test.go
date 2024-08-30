// Copyright 2019-2024 The Liqo Authors
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

package info

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

var _ = Describe("Info utilities functions tests", func() {
	Context("sPrintField function tests", func() {
		It("gets a string and a number from a struct", func() {
			o := Options{}
			type Dummy struct {
				Key string
				N   int
			}

			expectedVal := "value"
			expectedN := 48
			data := Dummy{Key: expectedVal, N: expectedN}

			By("getting a string value with a dotted query")
			checker := &dummyChecker{data: data, id: "dummy"}
			text, err := o.sPrintField("dummy.key", []Checker{checker}, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(expectedVal), "Unexpected string retrieved")

			By("getting a number value with a dotted query")
			text, err = o.sPrintField("dummy.n", []Checker{checker}, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(fmt.Sprint(expectedN)), "Unexpected number retrieved")

			By("getting a string value with a dotted query (trailing dot)")
			text, err = o.sPrintField(".dummy.key", []Checker{checker}, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(expectedVal), "Unexpected string retrieved")

			By("getting a string value with a dotted query (capitalized field)")
			text, err = o.sPrintField(".dummy.KEY", []Checker{checker}, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(expectedVal), "Unexpected string retrieved")

			By("getting a string with query shortcut")
			text, err = o.sPrintField("key", []Checker{checker}, map[string]string{"key": "dummy.key"})
			Expect(err).To(BeNil())
			Expect(text).To(Equal(expectedVal), "Unexpected string retrieved")
		})

		It("gets more complex data structures", func() {
			o := Options{Format: YAML}
			type NestedDummy struct {
				Key1 string
				Key2 string
			}

			type Dummy struct {
				Nested NestedDummy
				Slice  []string
				Map    map[string]string
			}

			expectedNested := NestedDummy{
				Key1: "hello",
				Key2: "Liqo",
			}
			expectedSlice := []string{"Slice", "Liqo"}
			expectedMap := map[string]string{"Map": "Liqo"}
			data := Dummy{
				Nested: expectedNested,
				Slice:  expectedSlice,
				Map:    expectedMap,
			}

			checker := &dummyChecker{data: data, id: "dummy"}

			By("getting a string a nested data structure")
			text, err := o.sPrintField("dummy.nested.key1", []Checker{checker}, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(expectedNested.Key1), "Unexpected string retrieved")

			By("getting the entire nested struct (YAML)")
			text, err = o.sPrintField("dummy.nested", []Checker{checker}, nil)
			expectedNestedYaml, _ := yaml.Marshal(expectedNested)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(
				string(expectedNestedYaml),
			), "Unexpected YAML data structure")

			By("getting a slice (YAML)")
			text, err = o.sPrintField("dummy.slice", []Checker{checker}, nil)
			expectedSliceYaml, _ := yaml.Marshal(expectedSlice)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(
				string(expectedSliceYaml),
			), "Unexpected YAML when getting a slice")

			By("getting a map (YAML)")
			text, err = o.sPrintField("dummy.map", []Checker{checker}, nil)
			expectedMapYaml, _ := yaml.Marshal(expectedMap)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(
				string(expectedMapYaml),
			), "Unexpected YAML when getting a map")

			By("getting the entire nested struct changin output to JSON")
			o.Format = JSON
			text, err = o.sPrintField("dummy.nested", []Checker{checker}, nil)
			expectedNestedJSON, _ := json.MarshalIndent(expectedNested, "", "  ")
			Expect(err).To(BeNil())
			Expect(text).To(Equal(
				string(expectedNestedJSON),
			), "Unexpected JSON data structure")
		})

		It("tries to get invalid fields", func() {
			o := Options{}
			type Dummy struct {
				Key string
				N   int
			}

			data := Dummy{Key: "val", N: 11}
			checker := &dummyChecker{data: data, id: "dummy"}

			By("getting invalid field (first field of the query)")
			_, err := o.sPrintField("invalid", []Checker{checker}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			By("getting invalid field (second field of the query)")
			_, err = o.sPrintField("dummy.invalid", []Checker{checker}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			By("trying to access a subfield of a non-object field")
			_, err = o.sPrintField("dummy.key.subfield", []Checker{checker}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not an object"))
		})
	})

	Context("installationCheck function tests", func() {
		var ctx context.Context

		BeforeEach(func() {
			ctx = context.Background()
		})

		It("check whether the Liqo namespace exists (existing namespace)", func() {
			o := Options{Factory: factory.NewForLocal()}
			o.LiqoNamespace = "liqo"
			o.KubeClient = k8sfake.NewSimpleClientset(testutil.FakeLiqoNamespace(o.LiqoNamespace))

			err := o.installationCheck(ctx)
			Expect(err).NotTo(HaveOccurred(), "Existing Liqo namespace but failed check")
		})

		It("check whether the Liqo namespace exists (not existing namespace)", func() {
			o := Options{Factory: factory.NewForLocal()}
			o.LiqoNamespace = "fakens"
			o.KubeClient = k8sfake.NewSimpleClientset()
			o.Printer = output.NewFakePrinter(GinkgoWriter)

			err := o.installationCheck(ctx)
			Expect(err).To(HaveOccurred(), "Existing Liqo namespace but failed check")
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})
})
