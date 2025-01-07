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

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/unauthenticate"
)

const liqoctlUnauthenticateLongHelp = `Unauthenticate a pair of consumer and provider clusters.

This command deletes all authentication resources on both consumer and provider clusters.
In the consumer cluster, it deletes the control plane Identity.
In the provider cluster, it deletes the Tenant.
The execution is prevented if any ResourceSlice or VirtualNode associated with the provider cluster is found.

Examples:
  $ {{ .Executable }} unauthenticate --remote-kubeconfig <provider>
`

// newUnauthenticateCommand represents the unauthenticate command.
func newUnauthenticateCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := unauthenticate.NewOptions(f)
	options.RemoteFactory = factory.NewForRemote()

	var cmd = &cobra.Command{
		Use:     "unauthenticate",
		Aliases: []string{"unauth"},
		Short:   "Unauthenticate a pair of consumer and provider clusters",
		Long:    WithTemplate(liqoctlUnauthenticateLongHelp),
		Args:    cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			twoClustersPersistentPreRun(cmd, options.LocalFactory, options.RemoteFactory, factory.WithScopedPrinter)
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.RunUnauthenticate(ctx))
		},
	}

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 2*time.Minute, "Timeout for completion")
	cmd.PersistentFlags().BoolVar(&options.Wait, "wait", true, "Wait for the unauthentication to complete")

	options.LocalFactory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)
	options.RemoteFactory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)

	options.LocalFactory.AddLiqoNamespaceFlag(cmd.PersistentFlags())
	options.RemoteFactory.AddLiqoNamespaceFlag(cmd.PersistentFlags())

	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("namespace",
		completion.Namespaces(ctx, options.LocalFactory, completion.NoLimit)))
	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("remote-namespace",
		completion.Namespaces(ctx, options.RemoteFactory, completion.NoLimit)))

	return cmd
}
