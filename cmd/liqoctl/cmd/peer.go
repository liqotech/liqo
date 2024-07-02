// Copyright 2019-2024 The Liqo Authors
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
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	nwforge "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/peer"
)

const liqoctlPeerLongHelp = `Enable a peering towards a remote provider cluster.

In Liqo, a *peering* is a unidirectional resource and service consumption
relationship between two Kubernetes clusters, with one cluster (i.e., the
consumer) granted the capability to offload tasks in a remote cluster (i.e., the
provider), but not vice versa. Bidirectional peerings can be achieved through
their combination. The same cluster can play the role of provider and consumer
in multiple peerings.

This commands enables a peering towards a remote provider cluster, performing 
the following operations:
- [optional] ensure networking between the two clusters
- ensure authentication between the two clusters (Identity in consumer cluster,
  Tenant in provider cluster)
- [optional] create ResourceSlice in consumer cluster and wait for it to be 
  accepted by the provider cluster
- [optional] create VirtualNode in consumer cluster

Examples:
  $ {{ .Executable }} peer --remote-kubeconfig <provider>
  $ {{ .Executable }} peer --remote-kubeconfig <provider> --server-service-type NodePort
  $ {{ .Executable }} peer --remote-kubeconfig <provider> --cpu 2 --memory 4Gi --pods 10
  $ {{ .Executable }} peer --remote-kubeconfig <provider> --create-resource-slice false
  $ {{ .Executable }} peer --remote-kubeconfig <provider> --create-virtual-node false
`

func newPeerCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := peer.NewOptions(f)
	options.RemoteFactory = factory.NewForRemote()

	cmd := &cobra.Command{
		Use:   "peer",
		Short: "Enable a peering towards a remote cluster",
		Long:  WithTemplate(liqoctlPeerLongHelp),
		Args:  cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			twoClustersPersistentPreRun(cmd, options.LocalFactory, options.RemoteFactory, factory.WithScopedPrinter)
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.RunPeer(ctx))
		},
	}

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for peering completion")

	options.LocalFactory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)
	options.RemoteFactory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)

	options.LocalFactory.AddLiqoNamespaceFlag(cmd.PersistentFlags())
	options.RemoteFactory.AddLiqoNamespaceFlag(cmd.PersistentFlags())

	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("namespace",
		completion.Namespaces(ctx, options.LocalFactory, completion.NoLimit)))
	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("remote-namespace",
		completion.Namespaces(ctx, options.RemoteFactory, completion.NoLimit)))

	// Networking flags
	cmd.Flags().BoolVar(&options.NetworkingDisabled, "networking-disabled", false, "Disable networking between the two clusters")
	cmd.Flags().Var(options.ServerServiceType, "server-service-type",
		fmt.Sprintf("Service type of the Gateway Server. Default: %s", nwforge.DefaultGwServerServiceType))
	cmd.Flags().Int32Var(&options.ServerPort, "server-port", nwforge.DefaultGwServerPort,
		fmt.Sprintf("Port of the Gateway Server. Default: %d", nwforge.DefaultGwServerPort))
	cmd.Flags().IntVar(&options.MTU, "mtu", nwforge.DefaultMTU,
		fmt.Sprintf("MTU of the Gateway server and client. Default: %d", nwforge.DefaultMTU))

	runtime.Must(cmd.RegisterFlagCompletionFunc("server-service-type", completion.Enumeration(options.ServerServiceType.Allowed)))

	// Authentication flags
	cmd.Flags().BoolVar(&options.CreateResourceSlice, "create-resource-slice", true, "Create a ResourceSlice for the peering")
	cmd.Flags().StringVar(&options.ResourceSliceClass, "resource-slice-class", "default", "The class of the ResourceSlice")
	cmd.Flags().StringVar(&options.ProxyURL, "proxy-url", "", "The URL of the proxy to use for the communication with the remote cluster")

	// Offloading flags
	cmd.Flags().BoolVar(&options.CreateVirtualNode, "create-virtual-node", true, "Create a VirtualNode for the peering")
	cmd.Flags().StringVar(&options.CPU, "cpu", "", "The amount of CPU requested for the VirtualNode")
	cmd.Flags().StringVar(&options.Memory, "memory", "", "The amount of memory requested for the VirtualNode")
	cmd.Flags().StringVar(&options.Pods, "pods", "", "The amount of pods requested for the VirtualNode")

	return cmd
}
