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

package peeringuser

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/peering-user/userfactory"
)

const liqoctlDeletePeeringUserHelp = `elete an existing user with the permissions to peer with this cluster.

Delete a peering user, so that it will no longer be able to peer with this cluster from the cluster with the given Cluster ID.
The previous credentials will be invalidated, and cannot be used anymore, even if the user is recreated.

Examples:
  $ {{ .Executable }} delete peering-user --consumer-cluster-id=<cluster-id>`

// Delete deletes a user.
func (o *Options) Delete(ctx context.Context, options *rest.DeleteOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peering-user",
		Short: "Delete an existing user with the permissions to peer with this cluster",
		Long:  liqoctlDeletePeeringUserHelp,
		Args:  cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			o.deleteOptions = options
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleDelete(ctx))
		},
	}

	cmd.Flags().Var(&o.clusterID, "consumer-cluster-id", "The cluster ID of the cluster from which peering has been performed")

	runtime.Must(cmd.MarkFlagRequired("consumer-cluster-id"))

	return cmd
}

func (o *Options) handleDelete(ctx context.Context) error {
	opts := o.deleteOptions
	clusterID := liqov1beta1.ClusterID(*o.clusterID.ClusterID)

	if err := userfactory.RemovePermissions(ctx, opts.CRClient, clusterID); err != nil {
		wErr := fmt.Errorf("unable to delete peering user: %w", err)
		opts.Printer.Error.Println(wErr)
		return wErr
	}

	opts.Printer.Success.Printfln("Peering user for cluster with ID %q deleted successfully", clusterID)
	return nil
}
