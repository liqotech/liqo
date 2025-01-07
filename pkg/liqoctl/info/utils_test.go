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

package info

import (
	"context"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
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
			collectedData := o.collectDataFromCheckers([]Checker{checker})
			text, err := o.sPrintField("dummy.key", collectedData, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(expectedVal), "Unexpected string retrieved")

			By("getting a number value with a dotted query")
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			text, err = o.sPrintField("dummy.n", collectedData, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(fmt.Sprint(expectedN)), "Unexpected number retrieved")

			By("getting a string value with a dotted query (trailing dot)")
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			text, err = o.sPrintField(".dummy.key", collectedData, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(expectedVal), "Unexpected string retrieved")

			By("getting a string value with a dotted query (capitalized field)")
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			text, err = o.sPrintField(".dummy.KEY", collectedData, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(expectedVal), "Unexpected string retrieved")

			By("getting a string with query shortcut")
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			text, err = o.sPrintField("key", collectedData, map[string]string{"key": "dummy.key"})
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
			collectedData := o.collectDataFromCheckers([]Checker{checker})
			text, err := o.sPrintField("dummy.nested.key1", collectedData, nil)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(expectedNested.Key1), "Unexpected string retrieved")

			By("getting the entire nested struct (YAML)")
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			text, err = o.sPrintField("dummy.nested", collectedData, nil)
			expectedNestedYaml, _ := yaml.Marshal(expectedNested)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(
				string(expectedNestedYaml),
			), "Unexpected YAML data structure")

			By("getting a slice (YAML)")
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			text, err = o.sPrintField("dummy.slice", collectedData, nil)
			expectedSliceYaml, _ := yaml.Marshal(expectedSlice)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(
				string(expectedSliceYaml),
			), "Unexpected YAML when getting a slice")

			By("getting a map (YAML)")
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			text, err = o.sPrintField("dummy.map", collectedData, nil)
			expectedMapYaml, _ := yaml.Marshal(expectedMap)
			Expect(err).To(BeNil())
			Expect(text).To(Equal(
				string(expectedMapYaml),
			), "Unexpected YAML when getting a map")

			By("getting the entire nested struct changin output to JSON")
			o.Format = JSON
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			text, err = o.sPrintField("dummy.nested", collectedData, nil)
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
			collectedData := o.collectDataFromCheckers([]Checker{checker})
			_, err := o.sPrintField("invalid", collectedData, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			By("getting invalid field (second field of the query)")
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			_, err = o.sPrintField("dummy.invalid", collectedData, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			By("trying to access a subfield of a non-object field")
			collectedData = o.collectDataFromCheckers([]Checker{checker})
			_, err = o.sPrintField("dummy.key.subfield", collectedData, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not an object"))
		})
	})

	Context("getForeignClusters function tests", func() {
		var (
			clientBuilder   fake.ClientBuilder
			ctx             context.Context
			foreignClusters = []client.Object{
				testutil.FakeForeignCluster("cl01", &liqov1beta1.Modules{
					Networking:     liqov1beta1.Module{},
					Authentication: liqov1beta1.Module{},
					Offloading:     liqov1beta1.Module{},
				}),
				testutil.FakeForeignCluster("cl02", &liqov1beta1.Modules{
					Networking:     liqov1beta1.Module{},
					Authentication: liqov1beta1.Module{},
					Offloading:     liqov1beta1.Module{},
				}),
			}
			options Options
		)

		BeforeEach(func() {
			ctx = context.Background()
			clientBuilder = *fake.NewClientBuilder().WithScheme(scheme.Scheme)
			options = Options{Factory: factory.NewForLocal()}
			options.Printer = output.NewFakePrinter(GinkgoWriter)
			// Define the ForeignCluster CRs to be returned
			clientBuilder.WithObjects(foreignClusters...)
			options.CRClient = clientBuilder.Build()
		})

		It("check that only the requested clusters are retrieved", func() {
			var expectedClusterID liqov1beta1.ClusterID = "cl01"

			err := options.getForeignClusters(ctx, []string{string(expectedClusterID)})
			Expect(err).NotTo(HaveOccurred(), "An error occurred while getting foreign clusters")
			Expect(len(options.ClustersInfo)).To(Equal(1), "Expected one single cluster in list")
			Expect(options.ClustersInfo).To(HaveKey(expectedClusterID), "Unexpected cluster info content")
			Expect(options.ClustersInfo[expectedClusterID].Spec.ClusterID).To(Equal(expectedClusterID),
				"Unexpected ForeignCluster object in cluster info")

		})

		It("check that an error is raised if a cluster does not exist", func() {
			var expectedClusterID liqov1beta1.ClusterID = "notexisting"

			err := options.getForeignClusters(ctx, []string{string(expectedClusterID)})
			Expect(err).To(MatchError(ContainSubstring("not found in active")), "No errors raised when target cluster does not exist")
		})

		It("check that when no IDs are provided all the clusters are retrieved", func() {
			expectedClusterIDs := []liqov1beta1.ClusterID{"cl01", "cl02"}

			err := options.getForeignClusters(ctx, []string{})
			Expect(err).NotTo(HaveOccurred(), "An error occurred while getting foreign clusters")
			Expect(len(options.ClustersInfo)).To(Equal(len(expectedClusterIDs)), "Expected all the peers to be retrieved")

			for _, clusterID := range expectedClusterIDs {
				Expect(options.ClustersInfo).To(HaveKey(clusterID), "Unexpected cluster info content")
				Expect(options.ClustersInfo[clusterID].Spec.ClusterID).To(Equal(clusterID),
					"Unexpected ForeignCluster object in cluster info")
			}
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

	Context("Retrieving the cluster name from a query", func() {
		var options Options

		BeforeEach(func() {
			options = Options{Factory: factory.NewForLocal()}
		})

		It("checks that the proper clusterID and truncated query is returned", func() {
			clusters := []liqov1beta1.ClusterID{"cl01", "cl02"}
			query := "network.status"
			expectedCluster := "cl02"
			gotQuery, gotCluster := options.getClusterFromQuery(fmt.Sprintf("%s.%s", expectedCluster, query), clusters)

			Expect(query).To(Equal(gotQuery), "Unexpected query received")
			Expect(expectedCluster).To(Equal(gotCluster), "Unexpected cluster received")
		})

		It("check that query is NOT truncated and the proper cluster ID returned with a single cluster", func() {
			clusters := []liqov1beta1.ClusterID{"cl01"}
			query := "network.cidr"
			expectedCluster := "cl01"
			gotQuery, gotCluster := options.getClusterFromQuery(query, clusters)

			Expect(query).To(Equal(gotQuery), "Unexpected query received")
			Expect(expectedCluster).To(Equal(gotCluster), "Unexpected cluster received")
		})
	})

	Context("collectDataFromMultiClusterCheckers function test", func() {
		var options Options

		BeforeEach(func() {
			options = Options{Factory: factory.NewForLocal()}
		})

		It("Checks that data is collected correctly for each clusterID", func() {
			// There is no need to access to the ForeignCluster structs, so, for simplicity, keep them nil
			options.ClustersInfo = map[liqov1beta1.ClusterID]*liqov1beta1.ForeignCluster{
				"cl01": nil,
				"cl02": nil,
			}

			type Data1 struct {
				ClusterID liqov1beta1.ClusterID
			}
			type Data2 struct {
				shoe string
			}

			clusterIDs := []liqov1beta1.ClusterID{"cl01", "cl02"}
			checkers := []MultiClusterChecker{}
			expectedData := map[liqov1beta1.ClusterID]map[string]interface{}{}
			checker1Data := map[liqov1beta1.ClusterID]interface{}{}
			checker2Data := map[liqov1beta1.ClusterID]interface{}{}

			for i, clusterID := range clusterIDs {
				expectedData[clusterID] = map[string]interface{}{}
				data1 := Data1{clusterID}
				data2 := Data2{fmt.Sprintf("shirt %d", i)}

				expectedData[clusterID]["c1"] = data1
				expectedData[clusterID]["c2"] = data2
				checker1Data[clusterID] = data1
				checker2Data[clusterID] = data2
			}

			checkers = append(checkers,
				&dummyMultiClusterChecker{
					id:    "c1",
					title: "C1",
					data:  checker1Data,
				},
				&dummyMultiClusterChecker{
					id:    "c2",
					title: "C2",
					data:  checker2Data,
				},
			)

			data, err := options.collectDataFromMultiClusterCheckers(checkers)
			Expect(err).NotTo(HaveOccurred(), "An error occurred while collecting data")
			Expect(data).To(Equal(expectedData), "Unexpected collected data")
		})
	})
})
