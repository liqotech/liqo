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
	"github.com/liqotech/liqo/pkg/liqoctl/generate"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

const liqoctlGeneratePeerLongHelp = `Generate the command to execute on another cluster to peer with the local cluster.

Upon execution, this command retrieves the information concerning the local
cluster (i.e., authentication endpoint and token, cluster ID, ...) and generates
a command that can be executed on a *different* cluster to establish an out-of-band
outgoing peering towards the local cluster. Once established, the remote cluster
will get access to a slice of the current cluster, and have the possibility to
offload workloads through the virtual node abstraction.

Examples:
  $ {{ .Executable }} generate peer-command
or
  $ {{ .Executable }} generate peer-command --namespace liqo-system --only-command
`

func newGenerateCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate data/commands to perform additional operations",
		Long:  "Generate data/commands to perform additional operations.",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newGeneratePeerCommand(ctx, f))
	return cmd
}

func newGeneratePeerCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := &generate.Options{Factory: f, CommandName: liqoctl}
	cmd := &cobra.Command{
		Use:   "peer-command",
		Short: "Generate the command to execute on another cluster to peer with the local cluster",
		Long:  WithTemplate(liqoctlGeneratePeerLongHelp),
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(options.Run(ctx))
		},
	}

	cmd.Flags().BoolVar(&options.OnlyCommand, "only-command", false, "Print only the resulting peer command, for scripts usage (default false)")

	f.AddLiqoNamespaceFlag(cmd.Flags())
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc(factory.FlagNamespace, completion.Namespaces(ctx, f, completion.NoLimit)))
	return cmd
}
