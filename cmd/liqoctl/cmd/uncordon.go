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
	"time"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/uncordon"
)

const liqoctlUncordonTenantLongHelp = `Uncordon a tenant cluster.

This command allows to uncordon a tenant cluster, allowing it to receive and accept new resources. 
Resources provided by existing ResourceSlices can be accepted again.

Examples:
  $ {{ .Executable }} uncordon tenant my-tenant-name
`

// newUncordonCommand represents the uncordon command.
func newUncordonCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "uncordon",
		Short: "Uncordon a liqo resource",
		Long:  "Uncordon a liqo resource",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newUncordonTenantCommand(ctx, f))

	return cmd
}

// newUncordonTenantCommand represents the uncordon command.
func newUncordonTenantCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := uncordon.NewOptions(f)

	var cmd = &cobra.Command{
		Use:               "tenant",
		Short:             "Uncordon a tenant cluster",
		Long:              WithTemplate(liqoctlUncordonTenantLongHelp),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.Tenants(ctx, f, 1),

		PreRun: func(cmd *cobra.Command, args []string) {
			output.ExitOnErr(options.Printer.AskConfirm("uncordon", options.SkipConfirm))
		},

		Run: func(cmd *cobra.Command, args []string) {
			options.TenantName = args[0]
			output.ExitOnErr(options.RunUncordonTenant(ctx))
		},
	}

	options.Factory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for uncordon completion")

	return cmd
}
