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

package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/network"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

const liqoctlNetworkLongHelp = `Manage liqo networking.`

const liqoctlNetworkInitLongHelp = `Initialize the liqo networking between two clusters.`

func newNetworkCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := &network.Options{LocalFactory: f}
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage liqo networking",
		Long:  WithTemplate(liqoctlNetworkLongHelp),
		Args:  cobra.NoArgs,
	}

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for completion")
	cmd.PersistentFlags().BoolVar(&options.Wait, "wait", false, "Wait for completion")

	cmd.AddCommand(newNetworkInitCommand(ctx, options))
	return cmd
}

func newNetworkInitCommand(ctx context.Context, options *network.Options) *cobra.Command {
	options.RemoteFactory = factory.NewForRemote()

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the liqo networking between two clusters",
		Long:  WithTemplate(liqoctlNetworkInitLongHelp),
		Args:  cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			twoClustersPersistentPreRun(cmd, options.LocalFactory, options.RemoteFactory, factory.WithScopedPrinter)
		},

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(options.RunInit(ctx))
		},
	}

	options.LocalFactory.AddFlags(cmd.Flags(), cmd.RegisterFlagCompletionFunc)
	options.RemoteFactory.AddFlags(cmd.Flags(), cmd.RegisterFlagCompletionFunc)

	options.LocalFactory.AddNamespaceFlag(cmd.Flags())
	options.RemoteFactory.AddNamespaceFlag(cmd.Flags())

	options.LocalFactory.AddLiqoNamespaceFlag(cmd.Flags())
	options.RemoteFactory.AddLiqoNamespaceFlag(cmd.Flags())

	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("namespace",
		completion.Namespaces(ctx, options.LocalFactory, completion.NoLimit)))
	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("remote-namespace",
		completion.Namespaces(ctx, options.RemoteFactory, completion.NoLimit)))

	return cmd
}
