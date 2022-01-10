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

	"github.com/liqotech/liqo/pkg/liqoctl/add"
)

// installCmd represents the generateInstall command.
func newAddCommand(ctx context.Context) *cobra.Command {
	var addCmd = &cobra.Command{
		Use:   add.UseCommand,
		Short: add.LiqoctlAddShortHelp,
		Long:  add.LiqoctlAddLongHelp,
	}
	addCmd.AddCommand(newAddClusterCommand(ctx))
	return addCmd
}

func newAddClusterCommand(ctx context.Context) *cobra.Command {
	installArgs := &add.ClusterArgs{}
	var addClusterCmd = &cobra.Command{
		Use:          add.ClusterResourceName,
		Short:        add.LiqoctlAddShortHelp,
		Long:         add.LiqoctlAddLongHelp,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			installArgs.ClusterName = args[0]
			return add.HandleAddCommand(ctx, installArgs)
		},
	}
	addClusterCmd.Flags().StringVar(&installArgs.ClusterAuthURL, add.AuthURLFlagName, "",
		"the AuthURL of the target Foreign Cluster")
	addClusterCmd.Flags().StringVar(&installArgs.ClusterToken, add.ClusterTokenFlagName, "",
		"the ClusterToken of the target Foreign Cluster")
	addClusterCmd.Flags().StringVar(&installArgs.ClusterID, add.ClusterIDFlagName, "",
		"the ClusterID assigned of the target Foreign Cluster")
	addClusterCmd.Flags().StringVar(&installArgs.Namespace, add.ClusterLiqoNamespaceFlagName, add.ClusterLiqoNamespace,
		"the namespace where Liqo is installed on the cluster")
	utilruntime.Must(addClusterCmd.MarkFlagRequired(add.ClusterIDFlagName))
	utilruntime.Must(addClusterCmd.MarkFlagRequired(add.ClusterTokenFlagName))
	utilruntime.Must(addClusterCmd.MarkFlagRequired(add.AuthURLFlagName))
	return addClusterCmd
}
