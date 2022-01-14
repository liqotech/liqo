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

package install

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// This recursive function takes a map and a list of keys and visits a tree of nested maps
// using the keys in the order provided. At each iteration, if the number of non-visited keys
// is 1, the function returns the value associated to the last key, else if it is greater
// than 1, the function expects the value to be a map and a new recursive iteration happens.
// In case the key is not found, an empty string is returned.
// In case no keys are provided, an error is returned.
// Example:
// 		m := map[string]interface{}{
//			"first": map[string]interface{}{
// 				"second": map[string]interface{}{
// 					"third": "value",
// 				},
// 			},
// 		}
// 		ValueFor(m, "first", "second", "third") // returns "value", nil
// 		ValueFor(m, "first", "second") // returns map[string]interface{}{ "third": "value" }, nil
// 		ValueFor(m, "first", "third") // returns "", nil
// 		ValueFor(m) // returns nil, "At least one key is required"
func ValueFor(m map[string]interface{}, keys ...string) (val interface{}, err error) {
	var ok bool
	if len(keys) == 0 {
		return nil, fmt.Errorf("At least one key is required")
	} else if val, ok = m[keys[0]]; !ok {
		return "", nil
	} else if len(keys) == 1 {
		return val, nil
	} else if m, ok = val.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("The value for key %s is not map (expected to be a map)", keys[0])
	} else {
		return ValueFor(m, keys[1:]...)
	}
}

func TestInstallCommand(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Install Command")
}

var _ = Describe("Test the install command works as expected", func() {
	type testCase struct {
		provider                 string
		parameters               []string
		providerValue            string
		regionValue              string
		customLabelValue         string
		clusterNameValue         string
		enableAdvertisementValue bool
		enableDiscoveryValue     bool
		reservedSubnetsValue     []interface{}
	}

	DescribeTable("An install command is issued",
		func(tc testCase) {
			cmd := &cobra.Command{}
			cmd.PersistentFlags().Bool("enable-lan-discovery", false, "")
			cmd.PersistentFlags().String("cluster-labels", "", "")
			cmd.PersistentFlags().String("cluster-name", "default-cluster-name", "")
			cmd.PersistentFlags().String("reserved-subnets", "", "")
			cmd.PersistentFlags().Bool("generate-name", false, "")
			cmd.SetArgs(tc.parameters)
			Expect(cmd.Execute()).To(Succeed())

			providerInstance := getProviderInstance(tc.provider)
			Expect(providerInstance).NotTo(BeNil())
			err := providerInstance.PreValidateGenericCommandArguments(cmd.Flags())
			Expect(err).ToNot(HaveOccurred())
			err = providerInstance.ValidateCommandArguments(cmd.Flags())
			Expect(err).ToNot(HaveOccurred())
			err = providerInstance.PostValidateGenericCommandArguments("")
			Expect(err).ToNot(HaveOccurred())

			// Chart values
			chartValues := map[string]interface{}{
				"discovery": map[string]interface{}{
					"config": map[string]interface{}{
						"clusterLabels":       map[string]interface{}{},
						"clusterName":         "",
						"enableAdvertisement": false,
						"enableDiscovery":     false,
					},
				},
				"networkManager": map[string]interface{}{
					"config": map[string]interface{}{
						"reservedSubnets": []interface{}{},
					},
				},
			}

			// Common values
			enableLanDiscovery, err := cmd.Flags().GetBool("enable-lan-discovery")
			Expect(err).ToNot(HaveOccurred())
			clusterLabels, err := cmd.Flags().GetString("cluster-labels")
			Expect(err).ToNot(HaveOccurred())
			clusterLabelsVar := argsutils.StringMap{}
			err = clusterLabelsVar.Set(clusterLabels)
			Expect(err).ToNot(HaveOccurred())
			clusterLabelsMap := installutils.GetInterfaceMap(clusterLabelsVar.StringMap)
			commonValues := map[string]interface{}{
				"discovery": map[string]interface{}{
					"config": map[string]interface{}{
						"clusterLabels":       clusterLabelsMap,
						"enableAdvertisement": enableLanDiscovery,
						"enableDiscovery":     enableLanDiscovery,
					},
				},
			}

			// Provider values
			providerValues := make(map[string]interface{})
			providerInstance.UpdateChartValues(providerValues)

			// Merged values
			values, err := generateValues(chartValues, commonValues, providerValues)
			Expect(err).ToNot(HaveOccurred())

			// Test values over expected ones
			Expect(ValueFor(values, "discovery", "config", "clusterLabels", "liqo.io/provider")).To(Equal(tc.providerValue))
			Expect(ValueFor(values, "discovery", "config", "clusterName")).To(Equal(tc.clusterNameValue))
			Expect(ValueFor(values, "discovery", "config", "clusterLabels", "topology.liqo.io/region")).To(Equal(tc.regionValue))
			Expect(ValueFor(values, "discovery", "config", "clusterLabels", "liqo.io/my-label")).To(Equal(tc.customLabelValue))
			Expect(ValueFor(values, "discovery", "config", "enableAdvertisement")).To(Equal(tc.enableAdvertisementValue))
			Expect(ValueFor(values, "discovery", "config", "enableDiscovery")).To(Equal(tc.enableDiscoveryValue))
			Expect(ValueFor(values, "networkManager", "config", "reservedSubnets")).To(Equal(tc.reservedSubnetsValue))
		},
		Entry("Install Kind cluster with default parameters", testCase{
			"kind",
			[]string{},
			"kind",
			"",
			"",
			"default-cluster-name",
			true,
			true,
			[]interface{}{},
		}),
		Entry("Install Kind cluster with one cluster labels' key-value pair", testCase{
			"kind",
			[]string{"--cluster-labels=topology.liqo.io/region=eu-east"},
			"kind",
			"eu-east",
			"",
			"default-cluster-name",
			true,
			true,
			[]interface{}{},
		}),
		Entry("Install Kind cluster with cluster labels, auto-discovery disabled, cluster name and reserved subnets", testCase{
			"kind",
			[]string{
				"--cluster-labels=topology.liqo.io/region=eu-east,liqo.io/my-label=custom,liqo.io/provider=provider-1",
				"--enable-lan-discovery=false",
				"--cluster-name=cluster-1",
				"--reserved-subnets=10.20.30.0/24,10.20.31.0/24",
			},
			"provider-1",
			"eu-east",
			"custom",
			"cluster-1",
			false,
			false,
			[]interface{}{"10.20.30.0/24", "10.20.31.0/24"},
		}),
		Entry("Install Kubeadm cluster with default parameters", testCase{
			"kubeadm",
			[]string{},
			"kubeadm",
			"",
			"",
			"default-cluster-name",
			false,
			false,
			[]interface{}{},
		}),
		Entry("Install Kubeadm cluster with cluster labels, cluster name and reserved subnets", testCase{
			"kubeadm",
			[]string{
				"--cluster-labels=topology.liqo.io/region=eu-east,liqo.io/my-label=custom,liqo.io/provider=provider-1",
				"--cluster-name=cluster-1",
				"--reserved-subnets=10.20.30.0/24,10.20.31.0/24",
			},
			"provider-1",
			"eu-east",
			"custom",
			"cluster-1",
			false,
			false,
			[]interface{}{"10.20.30.0/24", "10.20.31.0/24"},
		}),
	)
})
