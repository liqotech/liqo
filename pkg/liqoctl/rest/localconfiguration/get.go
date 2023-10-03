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

package localconfiguration

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/args"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
)

const liqoctlGetLocalConfigongHelp = `Retrieve the local network configuration.`

// Get implements the get command.
func (o *Options) Get(ctx context.Context, options *rest.GetOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "yaml")

	o.getOptions = options

	cmd := &cobra.Command{
		Use:     "localconfiguration",
		Aliases: []string{"localconfig", "lc", "localconfigurations"},
		Short:   "Get the local network configuration",
		Long:    liqoctlGetLocalConfigongHelp,
		Args:    cobra.NoArgs,

		PreRun: func(cmd *cobra.Command, args []string) {
			options.OutputFormat = outputFormat.Value
			o.getOptions = options
		},

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(o.handleGet(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting VirtualNode resource, instead of applying it. Supported formats: json, yaml")

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))

	return cmd
}

func (o *Options) handleGet(ctx context.Context) error {
	opts := o.getOptions

	conf, err := forgeLocalConfiguration(ctx, opts.CRClient, opts.LiqoNamespace)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to forge local configuration: %w", err))
		return err
	}

	opts.Printer.CheckErr(o.output(conf))
	return nil
}

// forgeLocalConfiguration creates a local configuration starting from the cluster identity and the IPAM storage.
func forgeLocalConfiguration(ctx context.Context, cl client.Client, liqoNamespace string) (*networkingv1alpha1.Configuration, error) {
	clusterIdentity, err := liqoutils.GetClusterIdentityWithControllerClient(ctx, cl, liqoNamespace)
	if err != nil {
		return nil, fmt.Errorf("unable to get cluster identity: %w", err)
	}

	ipamStorage, err := liqogetters.GetIPAMStorageByLabel(ctx, cl, labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("unable to get IPAM storage: %w", err)
	}

	return &networkingv1alpha1.Configuration{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkingv1alpha1.ConfigurationKind,
			APIVersion: networkingv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterIdentity.ClusterName,
			Labels: map[string]string{
				discovery.ClusterIDLabel: clusterIdentity.ClusterID,
			},
		},
		Spec: networkingv1alpha1.ConfigurationSpec{
			Remote: networkingv1alpha1.ClusterConfig{
				CIDR: networkingv1alpha1.ClusterConfigCIDR{
					Pod:      networkingv1alpha1.CIDR(ipamStorage.Spec.PodCIDR),
					External: networkingv1alpha1.CIDR(ipamStorage.Spec.ExternalCIDR),
				},
			},
		},
	}, nil
}

// output implements the logic to output the local configuration.
func (o *Options) output(conf *networkingv1alpha1.Configuration) error {
	opts := o.getOptions
	var printer printers.ResourcePrinter
	switch opts.OutputFormat {
	case "yaml":
		printer = &printers.YAMLPrinter{}
	case "json":
		printer = &printers.JSONPrinter{}
	default:
		return fmt.Errorf("unsupported output format %q", opts.OutputFormat)
	}

	return printer.PrintObj(conf, os.Stdout)
}
