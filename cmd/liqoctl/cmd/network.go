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

	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/network"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

const liqoctlNetworkLongHelp = `Manage liqo networking.`

const liqoctlNetworkInitLongHelp = `Initialize the liqo networking between two clusters.

It generates all network configurations required to connect the two clusters.`

const liqoctlNetworkResetLongHelp = `Tear down all liqo networking between two clusters.

It disconnects the two clusters and remove network configurations generated with the *network init* command.`

const liqoctlNetworConnectLongHelp = `Connect two clusters using liqo networking.

This command creates the Gateways to connect the two clusters.
Run this command after inizialiting the network using the *network init* command.`

const liqoctlNetworkDisconnectLongHelp = `Disconnect two clusters.

It deletes the Gateways, but keeps the network configurations generated with the *network init* command.`

func newNetworkCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := network.NewOptions(f)
	options.RemoteFactory = factory.NewForRemote()

	cmd := &cobra.Command{
		Use:   "network",
		Short: "Manage liqo networking",
		Long:  WithTemplate(liqoctlNetworkLongHelp),
		Args:  cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
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
	cmd.AddCommand(newNetworkResetCommand(ctx, options))
	cmd.AddCommand(newNetworkConnectCommand(ctx, options))
	cmd.AddCommand(newNetworkDisconnectCommand(ctx, options))

	return cmd
}

func newNetworkInitCommand(ctx context.Context, options *network.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the liqo networking between two clusters",
		Long:  WithTemplate(liqoctlNetworkInitLongHelp),
		Args:  cobra.NoArgs,

		Run: func(_ *cobra.Command, _ []string) {
			err := options.RunInit(ctx)
			if err != nil {
				options.LocalFactory.Printer.CheckErr(
					fmt.Errorf("`network init` failed (error: %w). Issue `network reset` to cleanup the environment", err))
			}
			output.ExitOnErr(err)
		},
	}

	return cmd
}

func newNetworkResetCommand(ctx context.Context, options *network.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Tear down liqo networking between two clusters (disconnect and remove network configurations)",
		Long:  WithTemplate(liqoctlNetworkResetLongHelp),
		Args:  cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.LocalFactory.Printer.AskConfirm("reset", options.LocalFactory.SkipConfirm))
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.RunReset(ctx))
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

		Run: func(_ *cobra.Command, _ []string) {
			err := options.RunConnect(ctx)
			if err != nil {
				options.LocalFactory.Printer.CheckErr(
					fmt.Errorf("`network connect` failed (error: %w). Issue `network disconnect` to cleanup the environment", err))
			}
			output.ExitOnErr(err)
		},
	}

	// Server flags
	cmd.Flags().StringVar(&options.ServerGatewayType, "server-type", forge.DefaultGwServerType,
		"Type of Gateway Server. Leave empty to use default Liqo implementation of WireGuard")
	cmd.Flags().StringVar(&options.ServerTemplateName, "server-template-name", forge.DefaultGwServerTemplateName,
		"Name of the Gateway Server template")
	cmd.Flags().StringVar(&options.ServerTemplateNamespace, "server-template-namespace", "",
		"Namespace of the Gateway Server template")
	cmd.Flags().Var(options.ServerServiceType, "server-service-type",
		fmt.Sprintf("Service type of the Gateway Server. Default: %s."+
			" Note: use ClusterIP only if you know what you are doing and you have a proper network configuration",
			forge.DefaultGwServerServiceType))
	cmd.Flags().Int32Var(&options.ServerPort, "server-port", forge.DefaultGwServerPort,
		fmt.Sprintf("Port of the Gateway Server. Default: %d", forge.DefaultGwServerPort))
	cmd.Flags().Int32Var(&options.ServerNodePort, "node-port", 0,
		"Force the NodePort of the Gateway Server. Leave empty to let Kubernetes allocate a random NodePort")
	cmd.Flags().StringVar(&options.ServerLoadBalancerIP, "load-balancer-ip", "",
		"Force LoadBalancer IP of the Gateway Server. Leave empty to use the one provided by the LoadBalancer provider")

	// Client flags
	cmd.Flags().StringVar(&options.ClientGatewayType, "client-type", forge.DefaultGwClientType,
		"Type of Gateway Client. Leave empty to use default Liqo implementation of WireGuard")
	cmd.Flags().StringVar(&options.ClientTemplateName, "client-template-name", forge.DefaultGwClientTemplateName,
		"Name of the Gateway Client template")
	cmd.Flags().StringVar(&options.ClientTemplateNamespace, "client-template-namespace", "",
		"Namespace of the Gateway Client template")

	// Common flags
	cmd.Flags().IntVar(&options.MTU, "mtu", forge.DefaultMTU,
		fmt.Sprintf("MTU of the Gateway server and client. Default: %d", forge.DefaultMTU))
	cmd.Flags().BoolVar(&options.DisableSharingKeys, "disable-sharing-keys", false, "Disable the sharing of public keys between the two clusters")

	runtime.Must(cmd.RegisterFlagCompletionFunc("server-service-type", completion.Enumeration(options.ServerServiceType.Allowed)))

	return cmd
}

func newNetworkDisconnectCommand(ctx context.Context, options *network.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect two clusters",
		Long:  WithTemplate(liqoctlNetworkDisconnectLongHelp),
		Args:  cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.LocalFactory.Printer.AskConfirm("disconnect", options.LocalFactory.SkipConfirm))
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.RunDisconnect(ctx))
		},
	}

	return cmd
}
