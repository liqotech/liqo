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
	"time"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/uninstall"
)

const liqoctlUninstallLongHelp = `Uninstall Liqo from the selected cluster.

This command wraps the Helm command to uninstall Liqo from the selected cluster,
optionally removing all the associated CRDs (i.e., with the --purge flag).

Warning: due to current limitations, the uninstallation process might hang in
case peerings are still established, or namespaces are selected for offloading.
It is necessary to unpeer all clusters and unoffload all namespaces in advance.

Examples:
  $ {{ .Executable }} uninstall
or
  $ {{ .Executable }} uninstall --purge
`

// newUninstallCommand generates a new Command representing `liqoctl uninstall`.
func newUninstallCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := uninstall.Options{Factory: f}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Liqo from the selected cluster",
		Long:  WithTemplate(liqoctlUninstallLongHelp),
		Args:  cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.Printer.AskConfirm("uninstall", f.SkipConfirm))
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.Run(ctx))
		},
	}

	cmd.Flags().BoolVar(&options.Purge, "purge", false, "Whether to purge all Liqo CRDs from the cluster (default false)")
	cmd.Flags().DurationVar(&options.Timeout, "timeout", 10*time.Minute, "The timeout for the completion of the uninstallation process")

	f.AddLiqoNamespaceFlag(cmd.Flags())
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc(factory.FlagNamespace, completion.Namespaces(ctx, f, completion.NoLimit)))

	return cmd
}
