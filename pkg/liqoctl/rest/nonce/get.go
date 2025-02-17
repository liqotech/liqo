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

package nonce

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	authutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/utils"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

const liqoctlGetNonceLongHelp = `Get a Nonce.

The Nonce secret is used to authenticate the remote cluster to the local cluster.

Examples:
  $ {{ .Executable }} get nonce --remote-cluster-id remote-cluster-id`

// Get implements the get command.
func (o *Options) Get(ctx context.Context, options *rest.GetOptions) *cobra.Command {
	o.getOptions = options

	cmd := &cobra.Command{
		Use:     "nonce",
		Aliases: []string{},
		Short:   "Get a nonce",
		Long:    liqoctlGetNonceLongHelp,
		Args:    cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			o.getOptions = options

			o.namespaceManager = tenantnamespace.NewManager(options.KubeClient, options.CRClient.Scheme())
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(o.handleGet(ctx))
		},
	}

	cmd.Flags().Var(&o.clusterID, "remote-cluster-id", "The cluster ID of the remote cluster")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))

	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx,
		o.getOptions.Factory, completion.NoLimit)))

	return cmd
}

func (o *Options) handleGet(ctx context.Context) error {
	opts := o.getOptions

	tenantNs, err := o.namespaceManager.GetNamespace(ctx, o.clusterID.GetClusterID())
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to retrieve tenant namespace: %v", output.PrettyErr(err)))
		return err
	}
	tenantNsName := tenantNs.GetName()

	nonceValue, err := authutils.RetrieveNonce(ctx, opts.CRClient, o.clusterID.GetClusterID(), tenantNsName)
	if err != nil {
		opts.Printer.CheckErr(fmt.Errorf("unable to retrieve nonce: %v", output.PrettyErr(err)))
		return err
	}

	fmt.Print(string(nonceValue))

	return nil
}
