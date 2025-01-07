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

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/move"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/args"
)

const liqoctlMoveVolumeLongHelp = `Move a Liqo-managed PVC to a different node (i.e., cluster).

Liqo supports the offloading of *stateful workloads* through a storage fabric
leveraging a custom storage class. PVCs associated with the Liqo storage class
eventually trigger the creation of the corresponding PV in the cluster (either
local or remote) where its first consumer (i.e., pod) is scheduled on. Locality
constraints are automatically embedded in the resulting PV, to enforce each pod
to be scheduled on the cluster where the associated storage pools are available.

This command allows to *move* a volume created in a given cluster to a different
cluster, ensuring mounting pods will then be attracted in that location. This
process leverages Restic to backup the source data and restore it into a volume
in the target cluster. Warning: only PVCs not currently mounted by any pod can
be moved to a different cluster.

Examples:
  $ {{ .Executable }} move volume database01 --namespace foo --target-node worker-023
or
  $ {{ .Executable }} move volume database01 --namespace foo --target-node liqo-neutral-colt
      --containers-cpu-limits 1000m --containers-ram-limits 2Gi
`

// moveCmd represents the move command.
func newMoveCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "move",
		Short: "Move an object to a different cluster",
		Long:  "Move an object to a different cluster.",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newMoveVolumeCommand(ctx, f))
	return cmd
}

func newMoveVolumeCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := &move.Options{Factory: f, ResticPassword: utils.RandomString(16)}
	var containersCPURequests, containersCPULimits args.Quantity
	var containersRAMRequests, containersRAMLimits args.Quantity

	var cmd = &cobra.Command{
		Use:     "volume",
		Aliases: []string{"pvc"},
		Short:   "Move a Liqo-managed PVC to a different node (i.e., cluster)",
		Long:    WithTemplate(liqoctlMoveVolumeLongHelp),

		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.PVCs(ctx, f, 1),

		PreRun: func(_ *cobra.Command, _ []string) {
			options.ContainersCPURequests = containersCPURequests.Quantity
			options.ContainersCPULimits = containersCPULimits.Quantity
			options.ContainersRAMRequests = containersRAMRequests.Quantity
			options.ContainersRAMLimits = containersRAMLimits.Quantity
		},

		Run: func(_ *cobra.Command, args []string) {
			options.VolumeName = args[0]
			output.ExitOnErr(options.Run(ctx))
		},
	}

	f.AddNamespaceFlag(cmd.Flags())
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc(factory.FlagNamespace, completion.Namespaces(ctx, f, completion.NoLimit)))

	cmd.Flags().StringVar(&options.TargetNode, "target-node", "",
		"The target node (either physical or virtual) the PVC will be moved to")

	cmd.Flags().Var(&containersCPURequests, "containers-cpu-requests", "The CPU requests for the Restic containers")
	cmd.Flags().Var(&containersCPULimits, "containers-cpu-limits", "The CPU limits for the Restic containers")
	cmd.Flags().Var(&containersRAMRequests, "containers-ram-requests", "The RAM requests for the Restic containers")
	cmd.Flags().Var(&containersRAMLimits, "containers-ram-limits", "The RAM limits for the Restic containers")
	cmd.Flags().StringVar(&options.ResticServerImage, "restic-server-image", move.DefaultResticServerImage,
		"The Restic server image to use")
	cmd.Flags().StringVar(&options.ResticImage, "restic-image", move.DefaultResticImage,
		"The Restic image to use")

	f.Printer.CheckErr(cmd.MarkFlagRequired("target-node"))
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("target-node", completion.Nodes(ctx, f, completion.NoLimit)))

	return cmd
}
