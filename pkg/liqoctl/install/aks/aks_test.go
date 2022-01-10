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

package aks

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-07-01/containerservice"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/consts"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test AKS provider")
}

const (
	endpoint    = "https://example.com"
	podCIDR     = "10.0.0.0/16"
	serviceCIDR = "10.80.0.0/16"

	subscriptionID    = "subID"
	resourceGroupName = "test"
	resourceName      = "liqo"

	region = "region"
)

var _ = Describe("Extract elements from AKS", func() {

	It("test flags", func() {

		p := NewProvider().(*aksProvider)

		cmd := &cobra.Command{}

		GenerateFlags(cmd)
		cmd.Flags().String("cluster-name", "", "")
		cmd.Flags().Bool("generate-name", true, "")
		cmd.Flags().String("reserved-subnets", "", "")

		flags := cmd.Flags()
		Expect(flags.Set("subscription-id", subscriptionID)).To(Succeed())
		Expect(flags.Set("resource-group-name", resourceGroupName)).To(Succeed())
		Expect(flags.Set("resource-name", resourceName)).To(Succeed())

		Expect(p.ValidateCommandArguments(flags)).To(Succeed())

		Expect(p.subscriptionID).To(Equal(subscriptionID))
		Expect(p.resourceGroupName).To(Equal(resourceGroupName))
		Expect(p.resourceName).To(Equal(resourceName))

	})

	It("test parse values", func() {
		ctx := context.TODO()

		clusterOutput := &containerservice.ManagedCluster{
			Location: pointer.StringPtr(region),
			ManagedClusterProperties: &containerservice.ManagedClusterProperties{
				Fqdn: pointer.StringPtr(endpoint),
				NetworkProfile: &containerservice.NetworkProfile{
					NetworkPlugin: containerservice.NetworkPluginKubenet,
					PodCidr:       pointer.StringPtr(podCIDR),
					ServiceCidr:   pointer.StringPtr(serviceCIDR),
				},
				AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{
					{
						VnetSubnetID: nil,
					},
				},
			},
		}

		p := NewProvider().(*aksProvider)

		Expect(p.parseClusterOutput(ctx, clusterOutput)).To(Succeed())

		Expect(p.endpoint).To(Equal(endpoint))
		Expect(p.serviceCIDR).To(Equal(serviceCIDR))
		Expect(p.podCIDR).To(Equal(podCIDR))
		Expect(len(p.ReservedSubnets)).To(BeNumerically("==", 1))
		Expect(p.ReservedSubnets).To(ContainElement(defaultAksNodeCIDR))
		Expect(p.ClusterLabels).ToNot(BeEmpty())
		Expect(p.ClusterLabels[consts.ProviderClusterLabel]).To(Equal(providerPrefix))
		Expect(p.ClusterLabels[consts.TopologyRegionClusterLabel]).To(Equal(region))

	})

})
