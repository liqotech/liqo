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

package gke

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"google.golang.org/api/container/v1"

	"github.com/liqotech/liqo/pkg/consts"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test GKE provider")
}

const (
	endpoint    = "https://example.com"
	podCIDR     = "10.0.0.0/16"
	serviceCIDR = "10.80.0.0/16"

	credentialsPath = "path"
	projectID       = "id"
	zone            = "zone"
	clusterID       = "cluster-id"
)

var _ = Describe("Extract elements from GKE", func() {

	It("test flags", func() {

		p := NewProvider().(*gkeProvider)

		cmd := &cobra.Command{}

		GenerateFlags(cmd)
		cmd.Flags().String("cluster-name", "", "")
		cmd.Flags().Bool("generate-name", true, "")
		cmd.Flags().String("reserved-subnets", "", "")

		flags := cmd.Flags()
		Expect(flags.Set("credentials-path", credentialsPath)).To(Succeed())
		Expect(flags.Set("project-id", projectID)).To(Succeed())
		Expect(flags.Set("zone", zone)).To(Succeed())
		Expect(flags.Set("cluster-id", clusterID)).To(Succeed())

		Expect(p.ValidateCommandArguments(flags)).To(Succeed())

		Expect(p.credentialsPath).To(Equal(credentialsPath))
		Expect(p.projectID).To(Equal(projectID))
		Expect(p.zone).To(Equal(zone))
		Expect(p.clusterID).To(Equal(clusterID))

	})

	It("test parse values", func() {

		clusterOutput := &container.Cluster{
			Endpoint:         endpoint,
			ServicesIpv4Cidr: serviceCIDR,
			ClusterIpv4Cidr:  podCIDR,
			Location:         zone,
		}

		p := NewProvider().(*gkeProvider)

		p.parseClusterOutput(clusterOutput)

		Expect(p.endpoint).To(Equal(endpoint))
		Expect(p.serviceCIDR).To(Equal(serviceCIDR))
		Expect(p.podCIDR).To(Equal(podCIDR))

		Expect(p.ClusterLabels).ToNot(BeEmpty())
		Expect(p.ClusterLabels[consts.ProviderClusterLabel]).To(Equal(providerPrefix))
		Expect(p.ClusterLabels[consts.TopologyRegionClusterLabel]).To(Equal(zone))

	})

})
