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

	"github.com/liqotech/liqo/pkg/liqoctl/connect"
)

func newConnectCommand(ctx context.Context) *cobra.Command {
	var params = connect.Args{}

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connects two clusters using a vpn tunnel",
		Long: `When clusters are in private networks and the API server of those clusters can not be reached directly.
						The connect command is used to configure the liqo components on both clusters in order to create a vpn 
						tunnel between them and expose the API servers through a proxy in order to be reachable from the clusters
						using the vpn connection.
		`,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return params.Handler(ctx)
		},
	}
	cmd.Flags().StringVarP(&params.Cluster1Namespace, "namespace1", "", "liqo", "Namespace Liqo is running in cluster 1")
	cmd.Flags().StringVarP(&params.Cluster2Namespace, "namespace2", "", "liqo", "Namespace Liqo is running in cluster 2")
	cmd.Flags().StringVarP(&params.Cluster1Kubeconfig, "config1", "", "", "Kubeconfig of cluster 1")
	cmd.Flags().StringVarP(&params.Cluster2Kubeconfig, "config2", "", "", "Kubeconfig of cluster 2")

	return cmd
}
