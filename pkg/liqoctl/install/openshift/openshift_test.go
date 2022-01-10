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

package openshift

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1api "github.com/openshift/api/config/v1"
	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/consts"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test OpenShift provider")
}

var _ = Describe("Extract elements from OpenShift", func() {

	var (
		p *openshiftProvider
	)

	BeforeEach(func() {
		p = NewProvider().(*openshiftProvider)
	})

	It("test flags", func() {
		cmd := &cobra.Command{}

		GenerateFlags(cmd)
		cmd.Flags().String("cluster-name", "", "")
		cmd.Flags().Bool("generate-name", true, "")
		cmd.Flags().String("reserved-subnets", "", "")

		flags := cmd.Flags()
		Expect(p.ValidateCommandArguments(flags)).To(Succeed())

		Expect(p.ClusterLabels).ToNot(BeEmpty())
		Expect(p.ClusterLabels[consts.ProviderClusterLabel]).To(Equal(providerPrefix))
	})

	It("test parse values", func() {
		podCidr := "10.128.0.0/14"
		serviceCidr := "172.30.0.0/16"

		networkConfig := &configv1api.Network{
			Status: configv1api.NetworkStatus{
				ClusterNetwork: []configv1api.ClusterNetworkEntry{
					{
						CIDR: podCidr,
					},
				},
				ServiceNetwork: []string{serviceCidr},
			},
		}

		Expect(p.parseNetworkConfig(networkConfig)).To(Succeed())

		Expect(p.podCIDR).To(Equal(podCidr))
		Expect(p.serviceCIDR).To(Equal(serviceCidr))
	})

})
