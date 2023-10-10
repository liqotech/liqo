// Copyright 2019-2023 The Liqo Authors
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

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/network"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayclient"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayserver"
)

const liqoctlNetworkLongHelp = `Manage liqo networking.`

const liqoctlNetworkInitLongHelp = `Initialize the liqo networking between two clusters.`

const liqoctlNetworConnectLongHelp = `Connect two clusters using liqo networking.

Run this command after inizialiting the network using the *network init* command.`

func newNetworkCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := network.NewOptions(f)
	options.RemoteFactory = factory.NewForRemote()

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage liqo networking",
		Long:  WithTemplate(liqoctlNetworkLongHelp),
		Args:  cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			twoClustersPersistentPreRun(cmd, options.LocalFactory, options.RemoteFactory, factory.WithScopedPrinter)
		},
	}

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for completion")
	cmd.PersistentFlags().BoolVar(&options.Wait, "wait", false, "Wait for completion")

	options.LocalFactory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)
	options.RemoteFactory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)

	options.LocalFactory.AddNamespaceFlag(cmd.PersistentFlags())
	options.RemoteFactory.AddNamespaceFlag(cmd.PersistentFlags())

	options.LocalFactory.AddLiqoNamespaceFlag(cmd.PersistentFlags())
	options.RemoteFactory.AddLiqoNamespaceFlag(cmd.PersistentFlags())

	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("namespace",
		completion.Namespaces(ctx, options.LocalFactory, completion.NoLimit)))
	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("remote-namespace",
		completion.Namespaces(ctx, options.RemoteFactory, completion.NoLimit)))
	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("liqo-namespace",
		completion.Namespaces(ctx, options.LocalFactory, completion.NoLimit)))
	options.LocalFactory.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("remote-liqo-namespace",
		completion.Namespaces(ctx, options.RemoteFactory, completion.NoLimit)))

	cmd.AddCommand(newNetworkInitCommand(ctx, options))
	cmd.AddCommand(newNetworkConnectCommand(ctx, options))

	return cmd
}

func newNetworkInitCommand(ctx context.Context, options *network.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the liqo networking between two clusters",
		Long:  WithTemplate(liqoctlNetworkInitLongHelp),
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(options.RunInit(ctx))
		},
	}

	return cmd
}

func newNetworkConnectCommand(ctx context.Context, options *network.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect two clusters using liqo networking",
		Long:  WithTemplate(liqoctlNetworConnectLongHelp),
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(options.RunConnect(ctx))
		},
	}

	// Server flags
	cmd.Flags().StringVar(&options.ServerGatewayType, "server-type", gatewayserver.DefaultGatewayType,
		"Type of Gateway Server. Leave empty to use default Liqo implementation of WireGuard")
	cmd.Flags().StringVar(&options.ServerTemplateName, "server-template-name", gatewayserver.DefaultTemplateName,
		"Name of the Gateway Server template")
	cmd.Flags().StringVar(&options.ServerTemplateNamespace, "server-template-namespace", gatewayserver.DefaultTemplateNamespace,
		"Namespace of the Gateway Server template")
	cmd.Flags().Var(options.ServerServiceType, "server-service-type",
		fmt.Sprintf("Service type of the Gateway Server. Default: %s", gatewayserver.DefaultServiceType))
	cmd.Flags().Int32Var(&options.ServerPort, "server-port", gatewayserver.DefaultPort,
		fmt.Sprintf("Port of the Gateway Server. Default: %d", gatewayserver.DefaultPort))
	cmd.Flags().IntVar(&options.ServerMTU, "server-mtu", gatewayserver.DefaultMTU,
		fmt.Sprintf("MTU of the Gateway Server. Default: %d", gatewayserver.DefaultMTU))

	// Client flags
	cmd.Flags().StringVar(&options.ClientGatewayType, "client-type", gatewayclient.DefaultGatewayType,
		"Type of Gateway Client. Leave empty to use default Liqo implementation of WireGuard")
	cmd.Flags().StringVar(&options.ClientTemplateName, "client-template-name", gatewayclient.DefaultTemplateName,
		"Name of the Gateway Client template")
	cmd.Flags().StringVar(&options.ClientTemplateNamespace, "client-template-namespace", gatewayclient.DefaultTemplateNamespace,
		"Namespace of the Gateway Client template")
	cmd.Flags().IntVar(&options.ClientMTU, "client-mtu", gatewayclient.DefaultMTU,
		fmt.Sprintf("MTU of the Gateway Client. Default: %d", gatewayclient.DefaultMTU))

	// Common flags
	cmd.Flags().BoolVar(&options.DisableSharingKeys, "disable-sharing-keys", false, "Disable the sharing of public keys between the two clusters")
	cmd.Flags().BoolVar(&options.Proxy, "proxy", gatewayserver.DefaultProxy, "Enable proxy for the Gateway Server")

	runtime.Must(cmd.RegisterFlagCompletionFunc("server-service-type", completion.Enumeration(options.ServerServiceType.Allowed)))

	return cmd
}
