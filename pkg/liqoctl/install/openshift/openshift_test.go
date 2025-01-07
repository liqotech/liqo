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

package openshift

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1api "github.com/openshift/api/config/v1"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test OpenShift provider")
}

var _ = Describe("Extract elements from OpenShift", func() {
	var options Options

	BeforeEach(func() {
		options = Options{Options: &install.Options{}}
	})

	It("test parse values", func() {
		const (
			podCidr     = "10.128.0.0/14"
			serviceCidr = "172.30.0.0/16"
		)

		networkConfig := &configv1api.Network{
			Status: configv1api.NetworkStatus{
				ClusterNetwork: []configv1api.ClusterNetworkEntry{{CIDR: podCidr}},
				ServiceNetwork: []string{serviceCidr},
			},
		}

		Expect(options.parseNetworkConfig(networkConfig)).To(Succeed())
		Expect(options.PodCIDR).To(Equal(podCidr))
		Expect(options.ServiceCIDR).To(Equal(serviceCidr))
	})
})
