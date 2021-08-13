package eks

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	flag "github.com/spf13/pflag"
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

		flags := flag.NewFlagSet("test", flag.PanicOnError)

		GenerateFlags(flags)

		Expect(flags.Set("eks.region", region)).To(Succeed())
		Expect(flags.Set("eks.cluster-name", clusterName)).To(Succeed())
		Expect(flags.Set("eks.user-name", userName)).To(Succeed())
		Expect(flags.Set("eks.policy-name", policyName)).To(Succeed())

		Expect(p.ValidateCommandArguments(flags)).To(Succeed())

		Expect(p.region).To(Equal(region))
		Expect(p.clusterName).To(Equal(clusterName))
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

		resVpcID, err := p.parseClusterOutput(clusterOutput)
		Expect(err).To(Succeed())
		Expect(resVpcID).To(Equal(vpcID))

		Expect(p.endpoint).To(Equal(endpoint))
		Expect(p.serviceCIDR).To(Equal(serviceCIDR))

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
