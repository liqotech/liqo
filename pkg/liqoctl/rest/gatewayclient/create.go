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

package gatewayclient

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/printers"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	forge "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

const liqoctlCreateGatewayClientLongHelp = `Create a Gateway Client.

The GatewayClient resource is used to define a Gateway Client for the external network.

Examples:
  $ {{ .Executable }} create gatewayclient my-gw-client \
  --remote-cluster-id remote-cluster-id \
  --type networking.liqo.io/v1beta1/wggatewayclients`

// Create creates a GatewayClient.
func (o *Options) Create(ctx context.Context, options *rest.CreateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	o.createOptions = options

	cmd := &cobra.Command{
		Use:     "gatewayclient",
		Aliases: []string{"gatewayclients", "client", "clients", "gwc"},
		Short:   "Create a Gateway Client",
		Long:    liqoctlCreateGatewayClientLongHelp,
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
		"Output the resulting GatewayClient resource, instead of applying it. Supported formats: json, yaml")

	cmd.Flags().Var(&o.RemoteClusterID, "remote-cluster-id", "The cluster ID of the remote cluster")
	cmd.Flags().StringVar(&o.GatewayType, "type", forge.DefaultGwClientType, "Type of Gateway Client. Default: wireguard")
	cmd.Flags().StringVar(&o.TemplateName, "template-name", forge.DefaultGwClientTemplateName, "Name of the Gateway Client template")
	cmd.Flags().StringVar(&o.TemplateNamespace, "template-namespace", "", "Namespace of the Gateway Client template")
	cmd.Flags().IntVar(&o.MTU, "mtu", forge.DefaultMTU, "MTU of Gateway Client")
	cmd.Flags().StringSliceVar(&o.Addresses, "addresses", []string{}, "Addresses of Gateway Server")
	cmd.Flags().Int32Var(&o.Port, "port", 0, "Port of Gateway Server")
	cmd.Flags().StringVar(&o.Protocol, "protocol", forge.DefaultProtocol, "Gateway Protocol")
	cmd.Flags().BoolVar(&o.Wait, "wait", false, "Wait for the Gateway Client to be ready")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("addresses"))
	runtime.Must(cmd.MarkFlagRequired("port"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	opts := o.createOptions

	gwClient, err := forge.GatewayClient(opts.Namespace, &opts.Name, o.getForgeOptions())
	if err != nil {
		opts.Printer.CheckErr(err)
		return err
	}

	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output(gwClient))
		return nil
	}

	s := opts.Printer.StartSpinner("Creating gatewayclient")

	_, err = resource.CreateOrUpdate(ctx, opts.CRClient, gwClient, func() error {
		return forge.MutateGatewayClient(gwClient, o.getForgeOptions())
	})
	if err != nil {
		s.Fail("Unable to create gatewayclient: %v", output.PrettyErr(err))
		return err
	}
	s.Success("Gatewayclient created")

	if o.Wait {
		s = opts.Printer.StartSpinner("Waiting for gatewayclient to be ready")
		interval := 1 * time.Second
		if err := wait.PollUntilContextCancel(ctx, interval, false, func(context.Context) (done bool, err error) {
			var appliedGwClient networkingv1beta1.GatewayClient
			err = opts.CRClient.Get(ctx, types.NamespacedName{
				Namespace: gwClient.Namespace,
				Name:      gwClient.Name,
			}, &appliedGwClient)
			if err != nil {
				return false, err
			}

			return appliedGwClient.Status.ClientRef != nil, nil
		}); err != nil {
			s.Fail("Unable to wait for gatewayclient to be ready: %v", output.PrettyErr(err))
			return err
		}
		s.Success("gatewayclient is ready")
	}

	return nil
}

// output implements the logic to output the generated Gateway Client resource.
func (o *Options) output(gwClient *networkingv1beta1.GatewayClient) error {
	var outputFormat string
	switch {
	case o.createOptions != nil:
		outputFormat = o.createOptions.OutputFormat
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

	return printer.PrintObj(gwClient, os.Stdout)
}
