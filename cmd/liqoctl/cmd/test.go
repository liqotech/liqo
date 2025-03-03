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

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/test"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
)

const liqoctlTestNetworkLongHelp = `Launch network E2E tests.

WARNING: to run the tests you need to have kyverno installed on every cluster https://kyverno.io/docs/installation/methods/ .

This command allows to launch E2E tests, which are used to check the network functionalities between the clusters.
The command needs to be run on the cluster that will act as the consumer,
and it requires the kubeconfig of the remote cluster that will act as the providers.
The consumer cluster must be peered with the providers, previously using "{{ .Executable }} peer".


Examples:
  $ {{ .Executable }} test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3
or
  $ {{ .Executable }} test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3 --basic
or
  $ {{ .Executable }} test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3 --np-nodes all --np-ext --pod-np
or
  $ {{ .Executable }} test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3 --ip
or
  $ {{ .Executable }} test network --remote-kubeconfigs $HOME/.kube/config2,$HOME/.kube/config3 --lb
`

func newTestCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := test.NewOptions(f)
	var cmd = &cobra.Command{
		Use:   "test",
		Short: "Launch E2E tests",
		Long:  "Launch E2E tests",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newTestNetworkCommand(ctx, options))

	options.AddFlags(cmd.PersistentFlags())

	return cmd
}

// newTestNetworkCommand represents the test network command.
func newTestNetworkCommand(ctx context.Context, topts *test.Options) *cobra.Command {
	options := network.NewOptions(flags.NewOptions(topts))

	var cmd = &cobra.Command{
		Use:     "network",
		Aliases: []string{"net"},
		Short:   "Launch E2E tests for the network",
		Long:    WithTemplate(liqoctlTestNetworkLongHelp),
		Args:    cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			ctx, cancel := context.WithTimeout(ctx, topts.Timeout)
			defer cancel()
			if err := options.RunNetworkTest(ctx); err != nil {
				topts.LocalFactory.Printer.ExitWithMessage(output.PrettyErr(err))
			}
		},
	}

	options.Nopts.AddFlags(cmd.Flags())
	runtime.Must(cmd.RegisterFlagCompletionFunc(
		string(flags.FlagNamesNodeportNodes), completion.Enumeration(flags.NodePortNodesValues),
	))
	runtime.Must(cmd.MarkFlagRequired(string(flags.FlagNamesProvidersKubeconfigs)))

	return cmd
}
