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

package aks

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v6"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test AKS provider")
}

var _ = Describe("Extract elements from AKS", func() {
	var options Options

	BeforeEach(func() {
		options = Options{Options: &install.Options{
			CommonOptions: &install.CommonOptions{
				ClusterLabels: map[string]string{},
			},
		}}
	})

	It("test parse values", func() {
		const (
			endpoint    = "https://example.com"
			podCIDR     = "10.0.0.0/16"
			serviceCIDR = "10.80.0.0/16"
			region      = "region"
		)

		clusterOutput := &armcontainerservice.ManagedCluster{
			Location: ptr.To(region),
			Properties: &armcontainerservice.ManagedClusterProperties{
				Fqdn: ptr.To(endpoint),
				NetworkProfile: &armcontainerservice.NetworkProfile{
					NetworkPlugin: ptr.To(armcontainerservice.NetworkPluginKubenet),
					PodCidr:       ptr.To(podCIDR),
					ServiceCidr:   ptr.To(serviceCIDR),
				},
				AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
					ptr.To(armcontainerservice.ManagedClusterAgentPoolProfile{
						VnetSubnetID: nil,
					}),
				},
			},
		}

		Expect(options.parseClusterOutput(context.Background(), clusterOutput)).To(Succeed())

		Expect(options.APIServer).To(Equal(endpoint))
		Expect(options.PodCIDR).To(Equal(podCIDR))
		Expect(options.ServiceCIDR).To(Equal(serviceCIDR))

		Expect(len(options.ReservedSubnets)).To(BeNumerically("==", 1))
		Expect(options.ReservedSubnets).To(ContainElement("10.224.0.0/16"))
		Expect(options.ClusterLabels[consts.TopologyRegionClusterLabel]).To(Equal(region))
	})
})
