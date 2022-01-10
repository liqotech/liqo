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
	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/consts"
)

func TestFetchingParameters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test EKS provider")
}

const (
	endpoint    = "https://example.com"
	podCIDR     = "10.0.0.0/16"
	serviceCIDR = "10.80.0.0/16"

	vpcID = "vpc-id"

	region      = "region"
	clusterName = "clusterName"
	userName    = "liqouser"
	policyName  = "liqopolicy"
)

var _ = Describe("Extract elements from EKS", func() {

	It("test flags", func() {

		p := NewProvider().(*eksProvider)

		cmd := &cobra.Command{}

		GenerateFlags(cmd)
		cmd.Flags().String("cluster-name", "", "")
		cmd.Flags().Bool("generate-name", true, "")
		cmd.Flags().String("reserved-subnets", "", "")

		flags := cmd.Flags()
		Expect(flags.Set("region", region)).To(Succeed())
		Expect(flags.Set("eks-cluster-name", clusterName)).To(Succeed())
		Expect(flags.Set("user-name", userName)).To(Succeed())
		Expect(flags.Set("policy-name", policyName)).To(Succeed())

		Expect(p.ValidateCommandArguments(flags)).To(Succeed())

		Expect(p.region).To(Equal(region))
		Expect(p.eksClusterName).To(Equal(clusterName))
		Expect(p.iamLiqoUser.userName).To(Equal(userName))
		Expect(p.iamLiqoUser.policyName).To(Equal(policyName))

	})

	It("test parse values", func() {

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

		p := NewProvider().(*eksProvider)
		p.region = region

		resVpcID, err := p.parseClusterOutput(clusterOutput)
		Expect(err).To(Succeed())
		Expect(resVpcID).To(Equal(vpcID))

		Expect(p.endpoint).To(Equal(endpoint))
		Expect(p.serviceCIDR).To(Equal(serviceCIDR))
		Expect(p.ClusterLabels).ToNot(BeEmpty())
		Expect(p.ClusterLabels[consts.ProviderClusterLabel]).To(Equal(providerPrefix))
		Expect(p.ClusterLabels[consts.TopologyRegionClusterLabel]).To(Equal(region))

		vpcOutput := &ec2.DescribeVpcsOutput{
			Vpcs: []*ec2.Vpc{
				{
					CidrBlock: aws.String(podCIDR),
				},
			},
		}

		Expect(p.parseVpcOutput("id", vpcOutput)).To(Succeed())

		Expect(p.podCIDR).To(Equal(podCIDR))

	})

})
