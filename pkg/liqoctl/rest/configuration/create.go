// Copyright 2019-2023 The Liqo Authors
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/utils/args"
)

const liqoctlCreateConfigurationLongHelp = `Create a Configuration.

The Configuration resource is used to represent a remote cluster network configuration.

Examples:
  $ {{ .Executable }} create configuration my-cluster --cluster-id my-cluster-id \
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

		PreRun: func(cmd *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			options.Name = args[0]
			o.createOptions = options
		},

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(o.handleCreate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting VirtualNode resource, instead of applying it. Supported formats: json, yaml")

	cmd.Flags().StringVar(&o.ClusterID, "cluster-id", "", "The cluster ID of the remote cluster")
	cmd.Flags().Var(&o.PodCIDR, "pod-cidr", "The pod CIDR of the remote cluster")
	cmd.Flags().Var(&o.ExternalCIDR, "external-cidr", "The external CIDR of the remote cluster")
	cmd.Flags().BoolVar(&o.Wait, "wait", false, "Wait for the Configuration to be ready")

	runtime.Must(cmd.MarkFlagRequired("cluster-id"))
	runtime.Must(cmd.MarkFlagRequired("pod-cidr"))
	runtime.Must(cmd.MarkFlagRequired("external-cidr"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("cluster-id", completion.ClusterIDs(ctx,
		o.createOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleCreate(ctx context.Context) error {
	opts := o.createOptions
	if opts.OutputFormat != "" {
		opts.Printer.CheckErr(o.output())
		return nil
	}

	s := opts.Printer.StartSpinner("Creating configuration")

	conf := o.forgeConfiguration(o.ClusterID, o.PodCIDR.String(), o.ExternalCIDR.String())
	_, err := controllerutil.CreateOrUpdate(ctx, opts.CRClient, conf, func() error {
		o.mutateConfiguration(conf)
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
			var appliedConf networkingv1alpha1.Configuration
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

func (o *Options) forgeConfiguration(clusterID, podCIDR, externalCIDR string) *networkingv1alpha1.Configuration {
	conf := &networkingv1alpha1.Configuration{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1alpha1.ConfigurationKind,
			APIVersion: networkingv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.createOptions.Name,
			Namespace: o.createOptions.Namespace,
			Labels: map[string]string{
				discovery.ClusterIDLabel: clusterID,
			},
		},
	}
	o.mutateConfiguration(conf)
	return conf
}

func (o *Options) mutateConfiguration(conf *networkingv1alpha1.Configuration) {
	conf.Kind = networkingv1alpha1.ConfigurationKind
	conf.APIVersion = networkingv1alpha1.GroupVersion.String()
	if conf.Labels == nil {
		conf.Labels = make(map[string]string)
	}
	conf.Labels[discovery.ClusterIDLabel] = conf.Name
	conf.Spec.Remote.CIDR.Pod = networkingv1alpha1.CIDR(o.PodCIDR.String())
	conf.Spec.Remote.CIDR.External = networkingv1alpha1.CIDR(o.ExternalCIDR.String())
}

// output implements the logic to output the generated VirtualNode resource.
func (o *Options) output() error {
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

	conf := o.forgeConfiguration(o.ClusterID, o.PodCIDR.String(), o.ExternalCIDR.String())
	return printer.PrintObj(conf, os.Stdout)
}
