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

	"github.com/liqotech/liqo/pkg/liqoctl/remove"
)

// removeCmd represents the remove command.
func newRemoveCommand(ctx context.Context) *cobra.Command {
	var removeCmd = &cobra.Command{
		Use:   remove.UseCommand,
		Short: remove.LiqoctlRemoveShortHelp,
		Long:  remove.LiqoctlRemoveLongHelp,
	}
	removeCmd.AddCommand(newRemoveClusterCommand(ctx))
	return removeCmd
}

func newRemoveClusterCommand(ctx context.Context) *cobra.Command {
	removeArgs := &remove.ClusterArgs{}
	var removeClusterCmd = &cobra.Command{
		Use:   remove.ClusterResourceName,
		Short: remove.LiqoctlRemoveShortHelp,
		Long:  remove.LiqoctlRemoveLongHelp,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			removeArgs.ClusterName = args[0]
			return remove.HandleRemoveCommand(ctx, removeArgs)
		},
	}

	return removeClusterCmd
}
