// Copyright 2019-2022 The Liqo Authors
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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/liqoctl/move"
)

const (
	// liqoctlMoveShortHelp contains the short help string for liqoctl move command.
	liqoctlMoveShortHelp = "Move liqo volumes to other clusters"
	// liqoctlMoveLongHelp contains the Long help string for liqoctl move command.
	liqoctlMoveLongHelp = `Move liqo volumes to other clusters`
)

// moveCmd represents the move command.
func newMoveCommand(ctx context.Context) *cobra.Command {
	var moveCmd = &cobra.Command{
		Use:   "move",
		Short: liqoctlMoveShortHelp,
		Long:  liqoctlMoveLongHelp,
	}
	moveCmd.AddCommand(newMoveVolumeCommand(ctx))
	return moveCmd
}

func newMoveVolumeCommand(ctx context.Context) *cobra.Command {
	clusterArgs := &move.Args{}
	var moveVolumeCmd = &cobra.Command{
		Use:           "volume",
		Short:         liqoctlMoveShortHelp,
		Long:          liqoctlMoveLongHelp,
		Args:          cobra.MinimumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterArgs.VolumeName = args[0]
			return move.HandleMoveVolumeCommand(ctx, clusterArgs)
		},
	}

	moveVolumeCmd.Flags().StringVarP(&clusterArgs.Namespace, "namespace", "n", "",
		"the namespace where the target PVC is stored")
	moveVolumeCmd.Flags().StringVar(&clusterArgs.TargetNode, "node", "",
		"the target node where the PVC will be moved")
	moveVolumeCmd.Flags().StringVar(&clusterArgs.ResticPassword, "restic-password", "",
		"the restic password to be used to for the restic repository")

	utilruntime.Must(moveVolumeCmd.MarkFlagRequired("namespace"))
	utilruntime.Must(moveVolumeCmd.MarkFlagRequired("node"))
	return moveVolumeCmd
}
