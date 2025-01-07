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
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install eks command.
type Options struct {
	*install.Options

	region         string
	eksClusterName string
	iamUser        iamLiqoUser
}

type iamLiqoUser struct {
	userName   string
	policyName string

	accessKeyID     string
	secretAccessKey string
}

// New initializes a new Provider object.
func New(o *install.Options) install.Provider {
	return &Options{Options: o}
}

// Name returns the name of the provider.
func (o *Options) Name() string { return "eks" }

// Examples returns the examples string for the given provider.
func (o *Options) Examples() string {
	return `Examples:
  $ {{ .Executable }} install eks --eks-cluster-region us-east-2 --eks-cluster-name foo
or
  $ {{ .Executable }} install eks --eks-cluster-region us-east-2 --eks-cluster-name foo \
      --user-name custom --policy-name custom-policy --access-key-id *** --secret-access-key ***
`
}

// RegisterFlags registers the flags for the given provider.
func (o *Options) RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.region, "eks-cluster-region", "", "The EKS region where the cluster is running")
	cmd.Flags().StringVar(&o.eksClusterName, "eks-cluster-name", "", "The EKS cluster name of the cluster")

	cmd.Flags().StringVar(&o.iamUser.userName, "user-name", "liqo-cluster-user",
		"The username of the Liqo user (automatically created if no access keys are provided)")
	cmd.Flags().StringVar(&o.iamUser.policyName, "policy-name", "liqo-cluster-policy",
		"The name of the policy assigned to the Liqo user (optional)")

	cmd.Flags().StringVar(&o.iamUser.accessKeyID, "access-key-id", "", "The IAM AccessKeyID for the Liqo user (optional)")
	cmd.Flags().StringVar(&o.iamUser.secretAccessKey, "secret-access-key", "", "The IAM SecretAccessKey for the Liqo user (optional)")

	utilruntime.Must(cmd.MarkFlagRequired("eks-cluster-region"))
	utilruntime.Must(cmd.MarkFlagRequired("eks-cluster-name"))
}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(_ context.Context) error {
	o.Printer.Verbosef("EKS Region: %q", o.region)
	o.Printer.Verbosef("EKS ClusterName: %q", o.eksClusterName)

	// if the cluster name has not been provided, we default it to the cloud provider resource name.
	if o.ClusterID == "" {
		o.ClusterID = liqov1beta1.ClusterID(o.eksClusterName)
	}

	o.Printer.Verbosef("Liqo IAM username: %q", o.iamUser.userName)
	o.Printer.Verbosef("Liqo IAM policy name: %q", o.iamUser.policyName)

	storedAccessKeyID, storedSecretAccessKey, err := retrieveIamAccessKey(o.iamUser.userName)
	if err != nil {
		return fmt.Errorf("failed retrieving access keys from cache: %w", err)
	}

	if storedAccessKeyID != "" && o.iamUser.accessKeyID == "" {
		o.iamUser.accessKeyID = storedAccessKeyID
	}
	if storedSecretAccessKey != "" && o.iamUser.secretAccessKey == "" {
		o.iamUser.secretAccessKey = storedSecretAccessKey
	}

	sess, err := session.NewSession()
	if err != nil {
		return fmt.Errorf("failed connecting to the AWS APIs: %w", err)
	}

	if err = o.getClusterInfo(sess); err != nil {
		return fmt.Errorf("failed retrieving cluster information: %w", err)
	}

	if err = o.createIamIdentity(sess); err != nil {
		return fmt.Errorf("failed creating the Liqo IAM identity: %w", err)
	}

	return nil
}

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{
		"networking": map[string]interface{}{
			"gatewayTemplates": map[string]interface{}{
				"server": map[string]interface{}{
					"service": map[string]interface{}{
						"annotations": map[string]interface{}{
							"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
						},
					},
				},
			},
			"fabric": map[string]interface{}{
				"config": map[string]interface{}{
					"fullMasquerade": true,
				},
			},
		},

		"authentication": map[string]interface{}{
			"awsConfig": map[string]interface{}{
				"accessKeyId":     o.iamUser.accessKeyID,
				"secretAccessKey": o.iamUser.secretAccessKey,
				"region":          o.region,
				"clusterName":     o.eksClusterName,
			},
		},
	}
}
