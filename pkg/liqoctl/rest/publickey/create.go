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

package publickey

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/printers"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

const liqoctlCreatePublicKeyLongHelp = `Create a PublicKey.

The PublicKey resource is used to define a PublicKey for the external network.

Examples:
  $ {{ .Executable }} create publickey my-public-key --remote-cluster-id remote-cluster-id --type server --gateway-name my-gateway`

// Create creates a PublicKey.
func (o *Options) Create(ctx context.Context, options *rest.CreateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	o.createOptions = options

	cmd := &cobra.Command{
		Use:     "publickey",
		Aliases: []string{"publickeys", "publickeies"},
		Short:   "Create a Public Key",
		Long:    liqoctlCreatePublicKeyLongHelp,
		Args:    cobra.ExactArgs(1),

		PreRun: func(_ *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			options.Name = args[0]
			o.createOptions = options
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleCreate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting PublicKey resource, instead of applying it. Supported formats: json, yaml")

	cmd.Flags().Var(&o.RemoteClusterID, "remote-cluster-id", "The cluster ID of the remote cluster")
	cmd.Flags().BytesBase64Var(&o.PublicKey, "public-key", nil, "The public key to be used for the Gateway")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("public-key"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	opts := o.createOptions

	pubKey, err := forge.PublicKey(opts.Namespace, &opts.Name, o.RemoteClusterID.GetClusterID(), o.PublicKey)
	if err != nil {
		opts.Printer.CheckErr(err)
		return err
	}

	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output(pubKey))
		return nil
	}

	s := opts.Printer.StartSpinner("Creating publickey")

	_, err = resource.CreateOrUpdate(ctx, opts.CRClient, pubKey, func() error {
		return forge.MutatePublicKey(pubKey, o.RemoteClusterID.GetClusterID(), o.PublicKey)
	})
	if err != nil {
		s.Fail("Unable to create publickey: %v", output.PrettyErr(err))
		return err
	}
	s.Success("Publickey created")

	return nil
}

// output implements the logic to output the generated PublicKey resource.
func (o *Options) output(pubKey *networkingv1beta1.PublicKey) error {
	var outputFormat string
	switch {
	case o.createOptions != nil:
		outputFormat = o.createOptions.OutputFormat
	case o.generateOptions != nil:
		outputFormat = o.generateOptions.OutputFormat
	default:
		return fmt.Errorf("unable to determine output format")
	}
	var printer printers.ResourcePrinter
	switch outputFormat {
	case "yaml":
		printer = &printers.YAMLPrinter{}
	case "json":
		printer = &printers.JSONPrinter{}
	default:
		return fmt.Errorf("unsupported output format %q", outputFormat)
	}

	return printer.PrintObj(pubKey, os.Stdout)
}
