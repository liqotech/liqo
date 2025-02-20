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
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

const liqoctlGeneratePeeringUserHelp = `Generate a new user with the permissions to peer with this cluster.

This command generates a user with the minimum permissions to peer with this cluster, from the cluster with
the given cluster ID, and returns a kubeconfig to be used to create or destroy the peering.

Examples:
  $ {{ .Executable }} generate peering-user --consumer-cluster-id=<cluster-id>`

// Generate generates a Nonce.
func (o *Options) Generate(ctx context.Context, options *rest.GenerateOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peering-user",
		Short: "Generate a new user with the permissions to peer with this cluster",
		Long:  liqoctlGeneratePeeringUserHelp,
		Args:  cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			o.generateOptions = options

			o.namespaceManager = tenantnamespace.NewManager(options.KubeClient, options.CRClient.Scheme())
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleGenerate(ctx))
		},
	}

	cmd.Flags().Var(&o.clusterID, "consumer-cluster-id", "The cluster ID of the cluster from which peering will be performed")

	runtime.Must(cmd.MarkFlagRequired("consumer-cluster-id"))

	return cmd
}

func (o *Options) handleGenerate(ctx context.Context) error {
	opts := o.generateOptions

	clusterID := liqov1beta1.ClusterID(*o.clusterID.ClusterID)
	opts.Printer.Warning.Println("Note that this functionality is currently not supported in EKS clusters")
	opts.Printer.Warning.Println("Please take note of this kubeconfig as it is not stored.")
	opts.Printer.Warning.Printfln("Note that it can only be used to peer with this cluster from a cluster with ID %s", clusterID)

	tenantNs, err := o.namespaceManager.CreateNamespace(ctx, clusterID)
	if err != nil {
		wErr := fmt.Errorf("unable to create the tenant namespace: %w", err)
		opts.Printer.Error.Println(wErr)
		return wErr
	}

	spinner := opts.Printer.StartSpinner("Generating a user for peering with this cluster")
	kubeconfig, err := userfactory.GeneratePeerUser(ctx, clusterID, tenantNs.Name, opts.Factory)
	if err != nil {
		spinner.Fail(err)
		return err
	}
	spinner.Success("User generated successfully")

	fmt.Println(kubeconfig)
	return nil
}
