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

package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/authenticate"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

const liqoctlAuthenticateLongHelp = `Authenticate with a provider cluster.

This command allows a consumer cluster to communicate with a remote provider cluster
to obtain slices of resources from. At the end of the process, the consumer cluster will
be able to replicate ResourceSlices resources to the provider cluster, and to receive
an associated Identity to consume the provided resources. 

Examples:
  $ {{ .Executable }} authenticate --remote-kubeconfig <provider>
`

// newAuthenticateCommand represents the authenticate command.
func newAuthenticateCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := authenticate.NewOptions(f)
	options.RemoteFactory = factory.NewForRemote()

	var cmd = &cobra.Command{
		Use:     "authenticate",
		Aliases: []string{"auth"},
		Short:   "Authenticate with a provider cluster",
		Long:    WithTemplate(liqoctlAuthenticateLongHelp),
		Args:    cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			twoClustersPersistentPreRun(cmd, options.LocalFactory, options.RemoteFactory, factory.WithScopedPrinter)
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.RunAuthenticate(ctx))
		},
	}

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 2*time.Minute, "Timeout for completion")

	options.LocalFactory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)
	options.RemoteFactory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)

	options.LocalFactory.AddLiqoNamespaceFlag(cmd.PersistentFlags())
	options.RemoteFactory.AddLiqoNamespaceFlag(cmd.PersistentFlags())

	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("namespace",
		completion.Namespaces(ctx, options.LocalFactory, completion.NoLimit)))
	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("remote-namespace",
		completion.Namespaces(ctx, options.RemoteFactory, completion.NoLimit)))

	cmd.Flags().BoolVar(&options.InBand, "in-band", false, "Use in-band authentication. Use it only if required and if you know what you are doing")
	cmd.Flags().StringVar(&options.ProxyURL, "proxy-url", "", "The URL of the proxy to use for the communication with the remote cluster")

	return cmd
}
