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

package gatewayserver

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
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

const liqoctlCreateGatewayServerLongHelp = `Create a Gateway Server.

The GatewayServer resource is used to define a Gateway Server for the external network.

Examples:
  $ {{ .Executable }} create gatewayserver my-gw-server \
  --remote-cluster-id remote-cluster-id \
  --type networking.liqo.io/v1beta1/wggatewayservers --service-type LoadBalancer`

// Create creates a GatewayServer.
func (o *Options) Create(ctx context.Context, options *rest.CreateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	o.createOptions = options

	cmd := &cobra.Command{
		Use:     "gatewayserver",
		Aliases: []string{"gatewayservers", "server", "servers", "gws"},
		Short:   "Create a Gateway Server",
		Long:    liqoctlCreateGatewayServerLongHelp,
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
		"Output the resulting GatewayServer resource, instead of applying it. Supported formats: json, yaml")

	cmd.Flags().Var(&o.RemoteClusterID, "remote-cluster-id", "The cluster ID of the remote cluster")
	cmd.Flags().StringVar(&o.GatewayType, "type", forge.DefaultGwServerType,
		"Type of Gateway Server. Leave empty to use default Liqo implementation of WireGuard")
	cmd.Flags().StringVar(&o.TemplateName, "template-name", forge.DefaultGwServerTemplateName, "Name of the Gateway Server template")
	cmd.Flags().StringVar(&o.TemplateNamespace, "template-namespace", "", "Namespace of the Gateway Server template")
	cmd.Flags().Var(o.ServiceType, "service-type", fmt.Sprintf("Service type of Gateway Server. Default: %s", forge.DefaultGwServerServiceType))
	cmd.Flags().IntVar(&o.MTU, "mtu", forge.DefaultMTU, "MTU of Gateway Server")
	cmd.Flags().Int32Var(&o.Port, "port", forge.DefaultGwServerPort, "Port of Gateway Server")
	cmd.Flags().Int32Var(&o.NodePort, "node-port", 0,
		"Force the NodePort of the Gateway Server. Leave empty to let Kubernetes allocate a random NodePort")
	cmd.Flags().StringVar(&o.LoadBalancerIP, "load-balancer-ip", "",
		"Force LoadBalancer IP of the Gateway Server. Leave empty to use the one provided by the LoadBalancer provider")
	cmd.Flags().BoolVar(&o.Wait, "wait", false, "Wait for the Gateway Server to be ready")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("service-type", completion.Enumeration(o.ServiceType.Allowed)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	opts := o.createOptions

	gwServer, err := forge.GatewayServer(opts.Namespace, &opts.Name, o.getForgeOptions())
	if err != nil {
		opts.Printer.CheckErr(err)
		return err
	}

	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output(gwServer))
		return nil
	}

	s := opts.Printer.StartSpinner("Creating gatewayserver")

	_, err = resource.CreateOrUpdate(ctx, opts.CRClient, gwServer, func() error {
		return forge.MutateGatewayServer(gwServer, o.getForgeOptions())
	})
	if err != nil {
		s.Fail("Unable to create gatewayserver: %v", output.PrettyErr(err))
		return err
	}
	s.Success("Gatewayserver created")

	if o.Wait {
		s = opts.Printer.StartSpinner("Waiting for gatewayserver to be ready")
		interval := 1 * time.Second
		if err := wait.PollUntilContextCancel(ctx, interval, false, func(context.Context) (done bool, err error) {
			var appliedGwServer networkingv1beta1.GatewayServer
			err = opts.CRClient.Get(ctx, types.NamespacedName{
				Namespace: gwServer.Namespace,
				Name:      gwServer.Name,
			}, &appliedGwServer)
			if err != nil {
				return false, err
			}

			return appliedGwServer.Status.Endpoint != nil && appliedGwServer.Status.ServerRef != nil, nil
		}); err != nil {
			s.Fail("Unable to wait for gatewayserver to be ready: %v", output.PrettyErr(err))
			return err
		}
		s.Success("gatewayserver is ready")
	}

	return nil
}

// output implements the logic to output the generated Gateway Server resource.
func (o *Options) output(gwServer *networkingv1beta1.GatewayServer) error {
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

	return printer.PrintObj(gwServer, os.Stdout)
}
