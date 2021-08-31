package eks

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"

	"github.com/liqotech/liqo/pkg/consts"
)

// getClusterInfo retrieved information from the EKS cluster.
func (k *eksProvider) getClusterInfo(sess *session.Session) error {
	eksSvc := eks.New(sess, aws.NewConfig().WithRegion(k.region))

	describeCluster := &eks.DescribeClusterInput{
		Name: aws.String(k.clusterName),
	}

	describeClusterResult, err := eksSvc.DescribeCluster(describeCluster)
	if err != nil {
		return err
	}

	vpcID, err := k.parseClusterOutput(describeClusterResult)
	if err != nil {
		return err
	}

	ec2Svc := ec2.New(sess, aws.NewConfig().WithRegion(k.region))

	describeVpc := &ec2.DescribeVpcsInput{
		VpcIds: aws.StringSlice([]string{vpcID}),
	}

	describeVpcResult, err := ec2Svc.DescribeVpcs(describeVpc)
	if err != nil {
		return err
	}

	if err = k.parseVpcOutput(vpcID, describeVpcResult); err != nil {
		return err
	}

	return nil
}

func (k *eksProvider) parseClusterOutput(describeClusterResult *eks.DescribeClusterOutput) (vpcID string, err error) {
	if describeClusterResult.Cluster.Endpoint == nil {
		err := fmt.Errorf("the EKS cluster %v in region %v does not have a valid endpoint", k.clusterName, k.region)
		return "", err
	}
	k.endpoint = *describeClusterResult.Cluster.Endpoint

	if describeClusterResult.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr == nil {
		err := fmt.Errorf("the EKS cluster %v in region %v does not have a valid service CIDR", k.clusterName, k.region)
		return "", err
	}
	k.serviceCIDR = *describeClusterResult.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr

	if describeClusterResult.Cluster.ResourcesVpcConfig.VpcId == nil {
		err := fmt.Errorf("the EKS cluster %v in region %v does not have a valid VPC ID", k.clusterName, k.region)
		return "", err
	}

	k.clusterLabels[consts.TopologyRegionClusterLabel] = k.region

	return *describeClusterResult.Cluster.ResourcesVpcConfig.VpcId, nil
}

func (k *eksProvider) parseVpcOutput(vpcID string, describeVpcResult *ec2.DescribeVpcsOutput) error {
	vpcs := describeVpcResult.Vpcs
	switch len(vpcs) {
	case 1:
		break
	case 0:
		err := fmt.Errorf("no VPC found with id %v", vpcID)
		return err
	default:
		err := fmt.Errorf("multiple VPC found with id %v", vpcID)
		return err
	}
	k.podCIDR = *vpcs[0].CidrBlock

	return nil
}
