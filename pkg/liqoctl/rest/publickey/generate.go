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

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/utils/args"
)

const liqoctlGeneratePuclicKeyHelp = `Generate the PublicKey of a Gateway Server or Client to be applied to other clusters.`

// Generate generates a PublicKey.
func (o *Options) Generate(ctx context.Context, options *rest.GenerateOptions) *cobra.Command {
	outputFormat := args.NewEnum([]string{"json", "yaml"}, "yaml")

	o.generateOptions = options

	cmd := &cobra.Command{
		Use:     "publickey",
		Aliases: []string{"publickeys", "publickeies"},
		Short:   "Generate a Public Key",
		Long:    liqoctlGeneratePuclicKeyHelp,
		Args:    cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			options.OutputFormat = outputFormat.Value
			o.generateOptions = options
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleGenerate(ctx))
		},
	}

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output format of the resulting PublicKey resource. Supported formats: json, yaml")

	cmd.Flags().Var(o.GatewayType, "gateway-type", fmt.Sprintf("The type of gateway resource. Allowed values: %s", o.GatewayType.Allowed))
	cmd.Flags().StringVar(&o.GatewayName, "gateway-name", "", "The name of the gateway (server or client) to pull the PublicKey from")

	runtime.Must(cmd.MarkFlagRequired("gateway-type"))
	runtime.Must(cmd.MarkFlagRequired("gateway-name"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("gateway-type", completion.Enumeration(o.GatewayType.Allowed)))
	runtime.Must(cmd.RegisterFlagCompletionFunc("gateway-name", completion.Gateways(ctx, o.generateOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleGenerate(ctx context.Context) error {
	opts := o.generateOptions

	pubKey, err := forge.PublicKeyForRemoteCluster(ctx, opts.CRClient, opts.LiqoNamespace, opts.Namespace, o.GatewayName, o.GatewayType.Value)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to forge PublicKey for remote cluster %q: %w", o.RemoteClusterID, err))
		return err
	}

	opts.Printer.CheckErr(o.output(pubKey))
	return nil
}
