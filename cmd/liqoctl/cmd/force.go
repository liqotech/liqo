// Copyright 2019-2026 The Liqo Authors
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
//

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/force/unpeer"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/utils"
)

const liqoctlForceLongHelp = `Force actions on Liqo components and resources.

The force command allows you to override normal Liqo operations and execute
actions that might otherwise be blocked or require manual intervention.
This command provides mechanisms to forcefully manipulate Liqo resources
when standard operations are not sufficient or when immediate action is required.

Use with caution as force operations may bypass safety checks and could
potentially impact cluster stability or data consistency.

Examples:
  $ {{ .Executable }} force
`

const liqoctUnpeerForceLongHelp = `Force unpeer from a remote cluster.

This command forcefully terminates the peering relationship with a remote cluster,
bypassing normal unpeer procedures and safety checks. It is designed to handle
situations where the standard unpeer process fails or when the remote cluster
is unreachable or unresponsive.

The force unpeer operation will:
- Mark the ForeignCluster as permanently unreachable
- Clean up local resources associated with the peering
- Remove tenant namespaces

Use with caution as this operation cannot be undone and may leave resources
in an inconsistent state if the remote cluster is still accessible.

Examples:
  $ {{ .Executable }} force unpeer <cluster-id>
`

func newForceUnpeerCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := unpeer.NewOptions(f)

	cmd := &cobra.Command{
		Use:               "unpeer",
		Short:             "Force unpeer a cluster",
		Long:              liqoctUnpeerForceLongHelp,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ClusterIDs(ctx, f, 1),

		PreRun: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(f.Printer.AskConfirm(
				"force unpeer might leave the remote cluster in an inconsistent state and it",
				f.SkipConfirm),
			)
		},

		Run: func(_ *cobra.Command, args []string) {
			options.ClusterID = args[0]
			if options.ClusterID == "" {
				options.Printer.ExitWithMessage("Cluster ID must be specified")
			}
			output.ExitOnErr(options.RunForceUnpeer(ctx))
		},
	}

	return cmd
}

func newForceCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	maincmd := &cobra.Command{
		Use:   "force",
		Short: "Force actions on Liqo",
		Long:  liqoctlForceLongHelp,
		Args:  cobra.NoArgs,
	}

	utils.AddCommand(maincmd, newForceUnpeerCommand(ctx, f))

	return maincmd
}
