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

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/peerib"
	"github.com/liqotech/liqo/pkg/liqoctl/peeroob"
)

const liqoctlPeerLongHelp = `Enable a peering towards a remote cluster.

In Liqo, a *peering* is a unidirectional resource and service consumption
relationship between two Kubernetes clusters, with one cluster (i.e., the
consumer) granted the capability to offload tasks in a remote cluster (i.e., the
provider), but not vice versa. Bidirectional peerings can be achieved through
their combination. The same cluster can play the role of provider and consumer
in multiple peerings.

Liqo supports two peering approaches, respectively referred to as out-of-band
control-plane and in-band control-plane. In the *out-of-band* control plane
peering, the Liqo control plane traffic flows outside the VPN tunnel used for
cross-cluster pod-to-pod communication. With the *in-band* approach, on the other
hand, all control plane traffic flows inside the VPN tunnel. The latter approach
relaxes the connectivity requirements, although it requires access to both
clusters (i.e., kubeconfigs) to start the peering process and setup the VPN tunnel.
`

const liqoctlPeerOOBLongHelp = `Enable an out-of-band peering towards a remote cluster.

The out-of-band control plane peering is the standard peering approach, with the
Liqo control-plane traffic flowing outside the VPN tunnel interconnecting the
two clusters. The VPN tunnel is dynamically started in a later stage of the
peering process, and it is leveraged only for cross-cluster pods traffic.

This approach supports clusters under the control of different administrative
domains (i.e., only local cluster access is required), and it is characterized
by higher dynamism and resilience in case of reconfigurations. Yet, it requires
three different endpoints to be reachable from the pods running in the remote
cluster (i.e., the Liqo authentication service, the Liqo VPN endpoint and the
Kubernetes API server).

Examples:
  $ {{ .Executable }} peer out-of-band eternal-donkey --auth-url <auth-url> \
      --cluster-id <cluster-id> --auth-token <auth-token>
or
  $ {{ .Executable }} peer out-of-band nearby-malamute --auth-url <auth-url> \
      --cluster-id <cluster-id> --auth-token <auth-token> --namespace liqo-system

The command above can be generated executing the following from the target cluster:
  $ {{ .Executable }} generate peer-command
`

const liqoctlPeerIBLongHelp = `Enable an in-band peering towards a remote cluster.

The in-band control plane peering is an peering approach, characterized by all
Liqo control-plane traffic flowing inside the VPN tunnel interconnecting the
two clusters. The VPN tunnel is established by {{ .Executable }} before starting the
remainder of the peering process.

This approach relaxes the network requirements, requiring only mutual
reachability of the VPN endpoints. To negotiate the network parameters and
establish the VPN tunnel this command requires access to both clusters.

Examples:
  $ {{ .Executable }} peer in-band --remote-kubeconfig "~/kube/config-remote"
or
  $ {{ .Executable }} peer in-band --remote-context remote
or
  $ {{ .Executable }} peer in-band --kubeconfig "~/kube/config-local" --remote-kubeconfig "~/kube/config-remote"
or
  $ {{ .Executable }} peer in-band --context local --remote-context remote
or
  $ {{ .Executable }} peer in-band --kubeconfig "~/kube/config-local" --context local \
      --remote-kubeconfig "~/kube/config-remote" --remote-context remote \
      --namespace liqo-system --remote-namespace liqo
`

func newPeerCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peer",
		Short: "Enable a peering towards a remote cluster",
		Long:  liqoctlPeerLongHelp,
		Args:  cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, args []string) { singleClusterPersistentPreRun(cmd, f) },
	}

	cmd.AddCommand(newPeerOutOfBandCommand(ctx, f))
	cmd.AddCommand(newPeerInBandCommand(ctx, f))
	return cmd
}

func newPeerOutOfBandCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := &peeroob.Options{Factory: f}
	cmd := &cobra.Command{
		Use:     "out-of-band cluster-name",
		Aliases: []string{"oob"},
		Short:   "Enable an out-of-band peering towards a remote cluster",
		Long:    WithTemplate(liqoctlPeerOOBLongHelp),
		Args:    cobra.ExactArgs(1),

		RunE: func(cmd *cobra.Command, args []string) error {
			options.ClusterName = args[0]
			return options.Run(ctx)
		},
	}

	cmd.Flags().StringVar(&options.ClusterAuthURL, peeroob.AuthURLFlagName, "",
		"The authentication URL of the target remote cluster")
	cmd.Flags().StringVar(&options.ClusterToken, peeroob.ClusterTokenFlagName, "",
		"The authentication token of the target remote cluster")
	cmd.Flags().StringVar(&options.ClusterID, peeroob.ClusterIDFlagName, "",
		"The Cluster ID identifying the target remote cluster")

	f.AddLiqoNamespaceFlag(cmd.Flags())
	utilruntime.Must(cmd.RegisterFlagCompletionFunc(factory.FlagNamespace, completion.Namespaces(ctx, f, completion.NoLimit)))

	utilruntime.Must(cmd.MarkFlagRequired(peeroob.ClusterIDFlagName))
	utilruntime.Must(cmd.MarkFlagRequired(peeroob.ClusterTokenFlagName))
	utilruntime.Must(cmd.MarkFlagRequired(peeroob.AuthURLFlagName))

	return cmd
}

func newPeerInBandCommand(ctx context.Context, local *factory.Factory) *cobra.Command {
	remote := factory.NewForRemote()
	options := peerib.Options{LocalFactory: local, RemoteFactory: remote}

	cmd := &cobra.Command{
		Use:     "in-band",
		Aliases: []string{"ib"},
		Short:   "Enable an in-band peering towards a remote cluster",
		Long:    WithTemplate(liqoctlPeerIBLongHelp),

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			twoClustersPersistentPreRun(cmd, local, remote, factory.WithScopedPrinter)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(ctx)
		},
	}

	local.AddLiqoNamespaceFlag(cmd.Flags())
	remote.AddLiqoNamespaceFlag(cmd.Flags())
	remote.AddFlags(cmd.Flags(), cmd.RegisterFlagCompletionFunc)

	cmd.Flags().BoolVar(&options.Bidirectional, "bidirectional", false,
		"Whether to establish a bidirectional peering (i.e., also from remote to local) (default false)")

	utilruntime.Must(cmd.RegisterFlagCompletionFunc("namespace", completion.Namespaces(ctx, options.LocalFactory, completion.NoLimit)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("remote-namespace", completion.Namespaces(ctx, options.RemoteFactory, completion.NoLimit)))

	return cmd
}
