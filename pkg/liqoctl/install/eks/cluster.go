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
func (o *Options) getClusterInfo(sess *session.Session) error {
	eksSvc := eks.New(sess, aws.NewConfig().WithRegion(o.region))

	describeCluster := &eks.DescribeClusterInput{
		Name: aws.String(o.eksClusterName),
	}

	describeClusterResult, err := eksSvc.DescribeCluster(describeCluster)
	if err != nil {
		return fmt.Errorf("unable to get cluster %s details, %w", *describeCluster.Name, err)
	}

	vpcID, err := o.parseClusterOutput(describeClusterResult)
	if err != nil {
		return fmt.Errorf("unable to parse cluster output, %w", err)
	}

	ec2Svc := ec2.New(sess, aws.NewConfig().WithRegion(o.region))

	describeVpc := &ec2.DescribeVpcsInput{
		VpcIds: aws.StringSlice([]string{vpcID}),
	}

	describeVpcResult, err := ec2Svc.DescribeVpcs(describeVpc)
	if err != nil {
		return fmt.Errorf("unable to get VPC %s details, %w", vpcID, err)
	}

	if err = o.parseVpcOutput(vpcID, describeVpcResult); err != nil {
		return err
	}

	return nil
}

func (o *Options) parseClusterOutput(describeClusterResult *eks.DescribeClusterOutput) (vpcID string, err error) {
	if describeClusterResult.Cluster.Endpoint == nil {
		err := fmt.Errorf("the EKS cluster %v in region %v does not have a valid endpoint", o.eksClusterName, o.region)
		return "", err
	}
	o.APIServer = *describeClusterResult.Cluster.Endpoint

	if describeClusterResult.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr == nil {
		err := fmt.Errorf("the EKS cluster %v in region %v does not have a valid service CIDR", o.eksClusterName, o.region)
		return "", err
	}
	o.ServiceCIDR = *describeClusterResult.Cluster.KubernetesNetworkConfig.ServiceIpv4Cidr

	if describeClusterResult.Cluster.ResourcesVpcConfig.VpcId == nil {
		err := fmt.Errorf("the EKS cluster %v in region %v does not have a valid VPC ID", o.eksClusterName, o.region)
		return "", err
	}

	o.ClusterLabels[consts.TopologyRegionClusterLabel] = o.region

	return *describeClusterResult.Cluster.ResourcesVpcConfig.VpcId, nil
}

func (o *Options) parseVpcOutput(vpcID string, describeVpcResult *ec2.DescribeVpcsOutput) error {
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
	o.PodCIDR = *vpcs[0].CidrBlock

	return nil
}
