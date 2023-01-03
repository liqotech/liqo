// Copyright 2019-2023 The Liqo Authors
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

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
		options = Options{Options: &install.Options{ClusterLabels: map[string]string{}}}
	})

	It("test parse values", func() {
		const (
			endpoint    = "https://example.com"
			podCIDR     = "10.0.0.0/16"
			serviceCIDR = "10.80.0.0/16"
			region      = "region"
		)

		clusterOutput := &containerservice.ManagedCluster{
			Location: pointer.StringPtr(region),
			ManagedClusterProperties: &containerservice.ManagedClusterProperties{
				Fqdn: pointer.StringPtr(endpoint),
				NetworkProfile: &containerservice.NetworkProfile{
					NetworkPlugin: containerservice.NetworkPluginKubenet,
					PodCidr:       pointer.StringPtr(podCIDR),
					ServiceCidr:   pointer.StringPtr(serviceCIDR),
				},
				AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{{VnetSubnetID: nil}},
			},
		}

		Expect(options.parseClusterOutput(context.Background(), clusterOutput)).To(Succeed())

		Expect(options.APIServer).To(Equal(endpoint))
		Expect(options.PodCIDR).To(Equal(podCIDR))
		Expect(options.ServiceCIDR).To(Equal(serviceCIDR))

		Expect(len(options.ReservedSubnets)).To(BeNumerically("==", 1))
		Expect(options.ReservedSubnets).To(ContainElement("10.240.0.0/16"))
		Expect(options.ClusterLabels[consts.TopologyRegionClusterLabel]).To(Equal(region))
	})
})
