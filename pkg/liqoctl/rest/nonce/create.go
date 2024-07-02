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

package nonce

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/forge"
	authutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/utils"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/args"
)

const liqoctlCreateNonceLongHelp = `Create a Nonce.

The Nonce secret is used to authenticate the remote cluster to the local cluster.

Examples:
  $ {{ .Executable }} create nonce --remote-cluster-id remote-cluster-id`

// Create creates a Nonce.
func (o *Options) Create(ctx context.Context, options *rest.CreateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	o.createOptions = options

	cmd := &cobra.Command{
		Use:     "nonce",
		Aliases: []string{},
		Short:   "Create a nonce",
		Long:    liqoctlCreateNonceLongHelp,
		Args:    cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			options.OutputFormat = outputFormat.Value
			o.createOptions = options

			o.namespaceManager = tenantnamespace.NewManager(options.KubeClient)
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleCreate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting Nonce secret, with no additional output. Supported formats: json, yaml")

	cmd.Flags().Var(&o.clusterID, "remote-cluster-id", "The cluster ID of the remote cluster")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	opts := o.createOptions
	waiter := wait.NewWaiterFromFactory(opts.Factory)

	tenantNs, err := o.namespaceManager.CreateNamespace(ctx, o.clusterID.GetClusterID())
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to create tenant namespace: %v", output.PrettyErr(err)))
		return err
	}

	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output(tenantNs.GetName()))
		return nil
	}

	// Ensure the presence of the nonce secret.
	s := opts.Printer.StartSpinner("Creating nonce")
	if err := authutils.EnsureNonceSecret(ctx, opts.CRClient, o.clusterID.GetClusterID(), tenantNs.GetName()); err != nil {
		s.Fail(fmt.Sprintf("Unable to create nonce secret: %v", output.PrettyErr(err)))
		return err
	}
	s.Success("Nonce created")

	// Wait for secret to be filled with the nonce.
	if err := waiter.ForNonce(ctx, o.clusterID.GetClusterID(), false); err != nil {
		return err
	}

	// Retrieve nonce from secret.
	s = opts.Printer.StartSpinner("Retrieving nonce")
	nonceValue, err := authutils.RetrieveNonce(ctx, opts.CRClient, o.clusterID.GetClusterID())
	if err != nil {
		s.Fail(fmt.Sprintf("Unable to retrieve nonce: %v", output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Nonce retrieved: %s", string(nonceValue)))

	return nil
}

// output implements the logic to output the generated Nonce secret.
func (o *Options) output(tenantNamespace string) error {
	opts := o.createOptions
	var printer printers.ResourcePrinter
	switch opts.OutputFormat {
	case "yaml":
		printer = &printers.YAMLPrinter{}
	case "json":
		printer = &printers.JSONPrinter{}
	default:
		return fmt.Errorf("unsupported output format %q", opts.OutputFormat)
	}

	nonce := forge.Nonce(tenantNamespace)
	if err := forge.MutateNonce(nonce, o.clusterID.GetClusterID()); err != nil {
		return err
	}

	return printer.PrintObj(nonce, os.Stdout)
}
