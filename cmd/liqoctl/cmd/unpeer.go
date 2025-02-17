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
	"github.com/liqotech/liqo/pkg/liqoctl/unpeer"
)

const liqoctlUnpeerLongHelp = `Disable a peering towards a remote provider cluster.

Depending on the approach adopted to initially establish the peering towards a
remote cluster, the corresponding unpeer command performs the symmetrical
operations to tear the peering down.

This command disables a peering towards a remote provider cluster, causing
virtual nodes and associated resourceslices to be destroyed, and all
offloaded workloads to be rescheduled. The Identity and Tenant are respectively
removed from the consumer and provider clusters, and the networking between the
two clusters is destroyed.

The reverse peering, if any, is preserved, and the remote cluster can continue
offloading workloads to its virtual node representing the local cluster.

Examples:
  $ {{ .Executable }} unpeer --remote-kubeconfig <provider>
`

func newUnpeerCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := unpeer.NewOptions(f)
	options.RemoteFactory = factory.NewForRemote()

	cmd := &cobra.Command{
		Use:   "unpeer",
		Short: "Disable a peering towards a remote provider cluster",
		Long:  WithTemplate(liqoctlUnpeerLongHelp),
		Args:  cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			twoClustersPersistentPreRun(cmd, options.LocalFactory, options.RemoteFactory, factory.WithScopedPrinter)
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.RunUnpeer(ctx))
		},
	}

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for unpeering completion")
	cmd.PersistentFlags().BoolVar(&options.Wait, "wait", true, "Wait for resource to be deleted before returning")
	cmd.PersistentFlags().BoolVar(&options.DeleteNamespace, "delete-namespaces", false, "Delete the tenant namespace after unpeering")

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
