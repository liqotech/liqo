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
	"time"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/liqoctl/purge"
)

const (
	// liqoctlPurgeShortHelp contains the short help message for purge Liqoctl command.
	liqoctlPurgeShortHelp = "Purge the Liqo peering between two clusters"
	// liqoctlPurgeLongHelp contains the long help message for purge Liqoctl command.
	liqoctlPurgeLongHelp = `Purge the Liqo peering between two clusters.`

	// liqoctlPurgeKubeconfig1Help contains the help message for kubeconfig1 flag.
	liqoctlPurgeKubeconfig1Help = "The kubeconfig file for the first cluster"
	// liqoctlPurgeKubeconfig2Help contains the help message for kubeconfig2 flag.
	liqoctlPurgeKubeconfig2Help = "The kubeconfig file for the second cluster (required when remote-cluster is not provided)"
	// liqoctlPurgeTimeoutHelp contains the help message for timeout flag.
	liqoctlPurgeTimeoutHelp = "The timeout for the graceful unpeer"
	// liqoctlPurgeRemoteClusterHelp contains the help message for remote-cluster flag.
	liqoctlPurgeRemoteClusterHelp = "The name of the foreign cluster resource to purge (required when kubeconfig-2 is not provided) " +
		"NOTE: the resources will not be purged in the remote cluster. You may need to run this command also in that cluster"
)

// newPurgeCommand generates a new Command representing `liqoctl purge`.
func newPurgeCommand(ctx context.Context) *cobra.Command {
	var purgeArgs purge.Args

	var purgeCmd = &cobra.Command{
		Use:           "purge",
		Short:         liqoctlPurgeShortHelp,
		Long:          liqoctlPurgeLongHelp,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return purge.HandlePurgeCommand(ctx, &purgeArgs)
		},
	}

	purgeCmd.Flags().StringVar(&purgeArgs.Config1, "kubeconfig-1", "", liqoctlPurgeKubeconfig1Help)
	purgeCmd.Flags().StringVar(&purgeArgs.Config2, "kubeconfig-2", "", liqoctlPurgeKubeconfig2Help)
	purgeCmd.Flags().StringVar(&purgeArgs.RemoteCluster, "remote-cluster", "", liqoctlPurgeRemoteClusterHelp)
	purgeCmd.Flags().DurationVar(&purgeArgs.Timeout, "timeout", 1*time.Minute, liqoctlPurgeTimeoutHelp)

	utilruntime.Must(purgeCmd.MarkFlagRequired("kubeconfig-1"))

	// TODO
	/*utilruntime.Must(purgeCmd.RegisterFlagCompletionFunc("remote-cluster",
	func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		names, err := autocompletion.GetClusterNames(cmd.Context(), toComplete)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	}))*/

	return purgeCmd
}
