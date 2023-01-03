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

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/status"
	statuslocal "github.com/liqotech/liqo/pkg/liqoctl/status/local"
	statuspeer "github.com/liqotech/liqo/pkg/liqoctl/status/peer"
)

const liqoctlStatusLongHelp = `Show the status of Liqo.

Liqoctl provides a set of commands to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in a human-readable format.

This command shows information about the local cluster and checks the presence
and the sanity of the liqo namespace and the liqo pods.

Examples:
  $ {{ .Executable }} status
or
  $ {{ .Executable }} status --namespace liqo-system
`

const liqoctlStatusPeerHelp = `Show the status of peered clusters.

Liqoctl provides a set of commands to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in a human-readable format.

This command shows information about peered clusters.

Examples:
  $ {{ .Executable }} status peer
or
  $ {{ .Executable }} status peer cluster1
or
  $ {{ .Executable }} status peer cluster1 cluster2
or
  $ {{ .Executable }} status peer cluster1 cluster2 --namespace liqo-system --verbose
`

func newStatusCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := status.Options{Factory: f}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the status of Liqo",
		Long:  WithTemplate(liqoctlStatusLongHelp),
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, args []string) {
			options.Checkers = []status.Checker{
				status.NewNamespaceChecker(&options, false),
				statuslocal.NewPodChecker(&options),
				statuslocal.NewLocalInfoChecker(&options),
			}
			output.ExitOnErr(options.Run(ctx))
		},
	}

	f.AddLiqoNamespaceFlag(cmd.PersistentFlags())
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc(factory.FlagNamespace, completion.Namespaces(ctx, f, completion.NoLimit)))

	cmd.PersistentFlags().BoolVar(&options.Verbose, "verbose", false, "Show more information")

	cmd.AddCommand(newStatusPeerCommand(ctx, f, &options))

	return cmd
}

func newStatusPeerCommand(ctx context.Context, f *factory.Factory, options *status.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "peer <peer-name ...>",
		Aliases:           []string{"peers"},
		Short:             "Show the status of peered clusters",
		Long:              WithTemplate(liqoctlStatusPeerHelp),
		ValidArgsFunction: completion.ForeignClusters(ctx, f, completion.NoLimit),

		Run: func(cmd *cobra.Command, args []string) {
			options.Checkers = []status.Checker{
				status.NewNamespaceChecker(options, true),
				statuspeer.NewPeerInfoChecker(options, args...),
			}
			output.ExitOnErr(options.Run(ctx))
		},
	}

	return cmd
}
