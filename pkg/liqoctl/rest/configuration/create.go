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

package configuration

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

const liqoctlCreateConfigurationLongHelp = `Create a Configuration.

The Configuration resource is used to represent a remote cluster network configuration.

Examples:
  $ {{ .Executable }} create configuration my-cluster --remote-cluster-id remote-cluster-id \
  --pod-cidr 10.0.0.0/16 --external-cidr 10.10.0.0/16`

// Create creates a Configuration.
func (o *Options) Create(ctx context.Context, options *rest.CreateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	o.createOptions = options

	cmd := &cobra.Command{
		Use:     "configuration",
		Aliases: []string{"config", "configurations"},
		Short:   "Create a Configuration",
		Long:    liqoctlCreateConfigurationLongHelp,
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
		"Output format of the resulting Configuration resource. Supported formats: json, yaml")

	cmd.Flags().Var(&o.RemoteClusterID, "remote-cluster-id", "The cluster ID of the remote cluster")
	cmd.Flags().Var(&o.PodCIDR, "pod-cidr", "The pod CIDR of the remote cluster")
	cmd.Flags().Var(&o.ExternalCIDR, "external-cidr", "The external CIDR of the remote cluster")
	cmd.Flags().BoolVar(&o.Wait, "wait", false, "Wait for the Configuration to be ready")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("pod-cidr"))
	runtime.Must(cmd.MarkFlagRequired("external-cidr"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	opts := o.createOptions

	conf := forge.Configuration(o.createOptions.Name, o.createOptions.Namespace,
		o.RemoteClusterID.GetClusterID(), o.PodCIDR.String(), o.ExternalCIDR.String())

	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output(conf))
		return nil
	}

	s := opts.Printer.StartSpinner("Creating configuration")
	_, err := resource.CreateOrUpdate(ctx, opts.CRClient, conf, func() error {
		forge.MutateConfiguration(conf, o.RemoteClusterID.GetClusterID(), o.PodCIDR.String(), o.ExternalCIDR.String())
		return nil
	})
	if err != nil {
		s.Fail("Unable to create configuration: %v", output.PrettyErr(err))
		return err
	}
	s.Success("Configuration created")

	if o.Wait {
		s = opts.Printer.StartSpinner("Waiting for configuration to be ready")
		interval := 1 * time.Second
		if err := wait.PollUntilContextCancel(ctx, interval, false, func(context.Context) (done bool, err error) {
			var appliedConf networkingv1beta1.Configuration
			err = opts.CRClient.Get(ctx, types.NamespacedName{
				Namespace: conf.Namespace,
				Name:      conf.Name,
			}, &appliedConf)
			if err != nil {
				return false, err
			}

			return appliedConf.Status.Remote != nil, nil
		}); err != nil {
			s.Fail("Unable to wait for configuration to be ready: %v", output.PrettyErr(err))
			return err
		}
		s.Success("Configuration is ready")
	}

	return nil
}

// output implements the logic to output the generated Configuration resource.
func (o *Options) output(conf *networkingv1beta1.Configuration) error {
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

	return printer.PrintObj(conf, os.Stdout)
}
