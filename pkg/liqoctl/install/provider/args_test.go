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

package provider

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

var _ = Describe("Args", func() {
	Describe("getDefaultMTU() function", func() {
		When("provider does not exist", func() {
			It("should return an error", func() {
				mtu, err := getDefaultMTU("notExistingProvider")
				Expect(mtu).To(BeZero())
				Expect(err).To(MatchError(fmt.Errorf("mtu for provider notExistingProvider not found")))
			})
		})

		When("provider exists", func() {
			It("should succeed", func() {
				mtu, err := getDefaultMTU("aks")
				Expect(mtu).To(Equal(providersDefaultMTU["aks"]))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("parseCommonValues() function", func() {
		Context("setting mtu for network interfaces", func() {
			When("mtu is set to zero, falling back to default value for each provider", func() {
				It("should return the right mtu in the configuration map", func() {
					for _, provider := range Providers {
						config, _, err := parseCommonValues(provider, pointer.String(""), "", "", "", false, false, false, 0, 0)
						Expect(err).NotTo(HaveOccurred())
						netConfig := config["networkConfig"].(map[string]interface{})
						Expect(netConfig["mtu"]).To(BeNumerically("==", providersDefaultMTU[provider]))
					}
				})
			})

			When("the provider does not exist", func() {
				It("should return an error", func() {
					_, _, err := parseCommonValues("notExisting", pointer.String(""), "", "", "", false, false, false, 0, 0)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(fmt.Errorf("mtu for provider notExisting not found")))
				})
			})

			When("the mtu is set by the user", func() {
				It("should set the mtu", func() {
					var mtu float64 = 1340
					config, _, err := parseCommonValues("eks", pointer.String(""), "", "", "", false, false, false, mtu, 0)
					Expect(err).NotTo(HaveOccurred())
					netConfig := config["networkConfig"].(map[string]interface{})
					Expect(netConfig["mtu"]).To(BeNumerically("==", mtu))
				})
			})

		})
	})
})
