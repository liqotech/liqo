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

package gke

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/api/container/v1"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test GKE provider")
}

var _ = Describe("Extract elements from GKE", func() {
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
			zone        = "zone"
		)

		clusterOutput := &container.Cluster{
			Endpoint:         endpoint,
			ServicesIpv4Cidr: serviceCIDR,
			ClusterIpv4Cidr:  podCIDR,
			Location:         zone,
		}

		options.parseClusterOutput(clusterOutput)

		Expect(options.APIServer).To(Equal(endpoint))
		Expect(options.ServiceCIDR).To(Equal(serviceCIDR))
		Expect(options.PodCIDR).To(Equal(podCIDR))

		Expect(options.ClusterLabels).ToNot(BeEmpty())
		Expect(options.ClusterLabels[consts.TopologyRegionClusterLabel]).To(Equal(zone))
	})
})
