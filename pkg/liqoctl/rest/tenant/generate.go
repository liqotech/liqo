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

package tenant

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	authutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/utils"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/args"
)

const liqoctlGenerateConfigHelp = `Generate the Tenant resource to be applied on the remote provider cluster.

This commands generates a Tenant filled with all the authentication parameters needed to authenticate with the remote cluster.
It signs the nonce provided by the remote cluster and generates the CSR.
The Nonce can be provided as a flag or it can be retrieved from the secret in the tenant namespace (if existing).   

Examples:
  $ {{ .Executable }} generate tenant --remote-cluster-id remote-cluster-id --remote-cluster-name remote-cluster-name`

// Generate generates a Tenant.
func (o *Options) Generate(ctx context.Context, options *rest.GenerateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "yaml")

	o.generateOptions = options

	cmd := &cobra.Command{
		Use:     "tenant",
		Aliases: []string{"tenants"},
		Short:   "Generate a Tenant",
		Long:    liqoctlGenerateConfigHelp,
		Args:    cobra.NoArgs,

		PreRun: func(cmd *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			o.generateOptions = options

			o.namespaceManager = tenantnamespace.NewManager(options.KubeClient)
		},

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(o.handleGenerate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output format of the resulting Tenant resource. Supported formats: json, yaml")

	cmd.Flags().StringVar(&o.remoteClusterIdentity.ClusterID, "remote-cluster-id", "", "The ID of the remote cluster")
	cmd.Flags().StringVar(&o.remoteClusterIdentity.ClusterName, "remote-cluster-name", "", "The name of the remote cluster")
	cmd.Flags().StringVar(&o.nonce, "nonce", "", "The nonce to sign for the authentication with the remote cluster")
	cmd.Flags().StringVar(&o.proxyURL, "proxy-url", "", "The URL of the proxy to use for the communication with the remote cluster")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("remote-cluster-name"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx, o.generateOptions.Factory, completion.NoLimit)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-name", completion.ClusterNames(ctx, o.generateOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleGenerate(ctx context.Context) error {
	opts := o.generateOptions
	waiter := wait.NewWaiterFromFactory(opts.Factory)

	// Ensure tenant namespace exists
	tenantNs, err := o.namespaceManager.CreateNamespace(ctx, o.remoteClusterIdentity)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to create tenant namespace: %w", err))
		return err
	}

	// Ensure the presence of the signed nonce secret.
	err = authutils.EnsureSignedNonceSecret(ctx, opts.CRClient, o.remoteClusterIdentity.ClusterID, tenantNs.GetName(), &o.nonce)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to ensure signed nonce secret: %w", err))
	}

	// Wait for secret to be filled with the signed nonce.
	if err := waiter.ForSignedNonce(ctx, o.remoteClusterIdentity.ClusterID, true); err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to wait for nonce to be signed: %w", err))
		return err
	}

	// Retrieve signed nonce from secret.
	signedNonce, err := authutils.RetrieveSignedNonce(ctx, opts.CRClient, o.remoteClusterIdentity.ClusterID)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to retrieve signed nonce: %w", err))
		return err
	}

	// Forge tenant resource for the remote cluster and output it.
	localClusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, opts.CRClient, opts.LiqoNamespace)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("an error occurred while retrieving cluster identity: %v", output.PrettyErr(err)))
		return err
	}

	tenant, err := authutils.GenerateTenant(ctx, opts.CRClient, &localClusterIdentity, opts.LiqoNamespace, signedNonce, &o.proxyURL)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to generate tenant: %w", err))
		return err
	}

	opts.Printer.CheckErr(o.output(tenant))

	return nil
}
