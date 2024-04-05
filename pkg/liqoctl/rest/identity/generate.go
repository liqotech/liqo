// Copyright 2019-2024 The Liqo Authors
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

package identity

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

const liqoctlGenerateConfigHelp = `Generate the Identity resource to be applied on the remote consumer cluster.

The Identity is generated from the Tenant associated with the provided remote clusterID.
It is intended to be applied on the remote consumer cluster.
This command generates only Identities used by the Liqo control plane for authentication purposes (e.g., CRDReplicator).   

Examples:
  $ {{ .Executable }} generate identity --remote-cluster-id remote-cluster-id`

// Generate generates a Identity.
func (o *Options) Generate(ctx context.Context, options *rest.GenerateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "yaml")

	o.generateOptions = options

	cmd := &cobra.Command{
		Use:     "identity",
		Aliases: []string{"identities"},
		Short:   "Generate a Identity",
		Long:    liqoctlGenerateConfigHelp,
		Args:    cobra.NoArgs,

		PreRun: func(cmd *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			o.generateOptions = options
		},

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(o.handleGenerate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output format of the resulting Identity resource. Supported formats: json, yaml")

	cmd.Flags().StringVar(&o.remoteClusterID, "remote-cluster-id", "", "The ID of the remote cluster")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx, o.generateOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleGenerate(ctx context.Context) error {
	opts := o.generateOptions

	// Get tenant with the given remote clusterID.
	tenant, err := getters.GetTenantByClusterID(ctx, opts.CRClient, o.remoteClusterID)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving tenant: %v", output.PrettyErr(err)))
		return err
	}

	if tenant.Status.AuthParams == nil || tenant.Status.TenantNamespace == "" {
		err := fmt.Errorf("tenant %s does not have the required status fields", tenant.Name)
		opts.Printer.CheckErr(err)
		return err
	}

	// Forge Identity resource for the remote cluster and output it.
	authParams := authv1alpha1.AuthParams{
		CA:        tenant.Status.AuthParams.CA,
		SignedCRT: tenant.Status.AuthParams.SignedCRT,
		APIServer: tenant.Status.AuthParams.APIServer,
		ProxyURL:  tenant.Spec.ProxyURL,

		AwsConfig: tenant.Status.AuthParams.AwsConfig,
	}

	localClusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, opts.CRClient, opts.LiqoNamespace)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving cluster identity: %v", output.PrettyErr(err)))
		return err
	}

	identity := forge.IdentityForRemoteCluster(localClusterIdentity, authv1alpha1.ControlPlaneIdentityType,
		&authParams, &tenant.Status.TenantNamespace)

	opts.Printer.CheckErr(o.output(identity))

	return nil
}
