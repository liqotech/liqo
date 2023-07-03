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
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/unpeerib"
	"github.com/liqotech/liqo/pkg/liqoctl/unpeeroob"
)

const liqoctlUnpeerLongHelp = `Disable a peering towards a remote cluster.

Depending on the approach adopted to initially establish the peering towards a
remote cluster, the corresponding unpeer command performs the symmetrical
operations to tear the peering down.

This command disables an *outgoing peering* towards a remote cluster, causing
the local virtual node (abstracting the remote cluster) to be destroyed, and all
offloaded workloads to be rescheduled. The reverse peering, if any, is preserved,
and the remote cluster can continue offloading workloads to its virtual node
representing the local cluster.

The same operation can be executed regardless of whether the peering is
out-of-band or in-band.

Examples:
  $ {{ .Executable }} unpeer eternal-donkey
`

const liqoctlUnpeerOOBLongHelp = `Disable an out-of-band peering towards a remote cluster.

This command disables an *out-of-band outgoing peering* towards a remote cluster,
causing the local virtual node (abstracting the remote cluster) to be destroyed,
and all offloaded workloads to be rescheduled. In addition, it attempts to remove
the foreign cluster resource, giving up and issuing a warning if an incoming
peering is still active.

In case the peering needs to be disabled only in one direction, while preserving
the other, it is possible to leverage the *unpeer <cluster-name>* command.

Examples:
  $ {{ .Executable }} unpeer out-of-band eternal-donkey
`

const liqoctlUnpeerIBLongHelp = `Disable an in-band peering towards a remote cluster.

This command disables an *in-band peering* towards a remote cluster, causing
both virtual nodes (if present) to be destroyed, all offloaded workloads to be
rescheduled, and finally tears down the cross-cluster VPN tunnel. At the end,
everything is restored to the same status as if the *peer in-band* command
towards that cluster had never been executed.

In case the peering needs to be disabled only in one direction, while preserving
the other, it is possible to leverage the *unpeer <cluster-name>* command.

Examples:
  $ {{ .Executable }} unpeer in-band --remote-kubeconfig "~/kube/config-remote"
or
  $ {{ .Executable }} unpeer in-band --remote-context remote
or
  $ {{ .Executable }} unpeer in-band --kubeconfig "~/kube/config-local" --remote-kubeconfig "~/kube/config-remote"
or
  $ {{ .Executable }} unpeer in-band --context local --remote-context remote
or
  $ {{ .Executable }} unpeer in-band --kubeconfig "~/kube/config-local" --context local \
      --remote-kubeconfig "~/kube/config-remote" --remote-context remote \
      --namespace liqo-system --remote-namespace liqo
`

func newUnpeerCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := &unpeeroob.Options{Factory: f}
	var cmd = &cobra.Command{
		Use:               "unpeer",
		Short:             "Disable a peering towards a remote cluster",
		Long:              WithTemplate(liqoctlUnpeerLongHelp),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ForeignClusters(ctx, f, 1),

		PreRun: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(options.Printer.AskConfirm("unpeer", options.SkipConfirm))
		},

		Run: func(cmd *cobra.Command, args []string) {
			options.ClusterName = args[0]
			output.ExitOnErr(options.Run(ctx))
		},
	}

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for unpeering completion")

	cmd.AddCommand(newUnpeerOutOfBandCommand(ctx, options))
	cmd.AddCommand(newUnpeerInBandCommand(ctx, options))
	return cmd
}

func newUnpeerOutOfBandCommand(ctx context.Context, options *unpeeroob.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "out-of-band cluster-name",
		Aliases: []string{"oob"},
		Short:   "Disable an out-of-band peering towards a remote cluster",
		Long:    WithTemplate(liqoctlUnpeerOOBLongHelp),

		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ForeignClusters(ctx, options.Factory, 1),

		PreRun: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(options.Printer.AskConfirm("unpeer", options.SkipConfirm))
		},

		Run: func(cmd *cobra.Command, args []string) {
			options.ClusterName = args[0]
			options.UnpeerOOBMode = true
			output.ExitOnErr(options.Run(ctx))
		},
	}

	return cmd
}

func newUnpeerInBandCommand(ctx context.Context, unpeerOptions *unpeeroob.Options) *cobra.Command {
	local := unpeerOptions.Factory
	remote := factory.NewForRemote()
	options := unpeerib.Options{LocalFactory: local, RemoteFactory: remote}

	cmd := &cobra.Command{
		Use:     "in-band",
		Aliases: []string{"ib"},
		Short:   "Disable an in-band peering towards a remote cluster",
		Long:    WithTemplate(liqoctlUnpeerIBLongHelp),
		Args:    cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			twoClustersPersistentPreRun(cmd, local, remote, factory.WithScopedPrinter)
		},

		PreRun: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(local.Printer.AskConfirm("unpeer", local.SkipConfirm))
		},

		Run: func(cmd *cobra.Command, args []string) {
			options.Timeout = unpeerOptions.Timeout
			output.ExitOnErr(options.Run(ctx))
		},
	}

	local.AddLiqoNamespaceFlag(cmd.Flags())
	remote.AddLiqoNamespaceFlag(cmd.Flags())
	remote.AddFlags(cmd.Flags(), cmd.RegisterFlagCompletionFunc)

	local.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("namespace", completion.Namespaces(ctx, options.LocalFactory, completion.NoLimit)))
	local.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("remote-namespace", completion.Namespaces(ctx, options.RemoteFactory, completion.NoLimit)))

	return cmd
}
