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

package eks

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test EKS provider")
}

var _ = Describe("Extract elements from EKS", func() {
	var options Options

	BeforeEach(func() {
		options = Options{Options: &install.Options{ClusterLabels: map[string]string{}}}
	})

	It("test parse values", func() {
		const (
			endpoint    = "https://example.com"
			podCIDR     = "10.0.0.0/16"
			serviceCIDR = "10.80.0.0/16"

			vpcID  = "vpc-id"
			region = "region"
		)

		clusterOutput := &eks.DescribeClusterOutput{
			Cluster: &eks.Cluster{
				Endpoint: aws.String(endpoint),
				KubernetesNetworkConfig: &eks.KubernetesNetworkConfigResponse{
					ServiceIpv4Cidr: aws.String(serviceCIDR),
				},
				ResourcesVpcConfig: &eks.VpcConfigResponse{
					VpcId: aws.String(vpcID),
				},
			},
		}

		options.region = region

		resVpcID, err := options.parseClusterOutput(clusterOutput)
		Expect(err).To(Succeed())
		Expect(resVpcID).To(Equal(vpcID))

		Expect(options.APIServer).To(Equal(endpoint))
		Expect(options.ServiceCIDR).To(Equal(serviceCIDR))
		Expect(options.ClusterLabels[consts.TopologyRegionClusterLabel]).To(Equal(region))

		vpcOutput := &ec2.DescribeVpcsOutput{
			Vpcs: []*ec2.Vpc{
				{
					CidrBlock: aws.String(podCIDR),
				},
			},
		}

		Expect(options.parseVpcOutput("id", vpcOutput)).To(Succeed())
		Expect(options.PodCIDR).To(Equal(podCIDR))
	})
})
