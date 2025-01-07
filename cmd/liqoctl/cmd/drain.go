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
	"github.com/liqotech/liqo/pkg/liqoctl/drain"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

const liqoctlDrainTenantLongHelp = `Drain a tenant cluster.

This command allows to drain a tenant cluster, preventing it from receiving new resources.
Resources provided by existing ResourceSlices are drained.

Examples:
  $ {{ .Executable }} drain tenant my-tenant-name
`

// newDrainCommand represents the drain command.
func newDrainCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "drain",
		Short: "Drain a liqo resource",
		Long:  "Drain a liqo resource",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newDrainTenantCommand(ctx, f))

	return cmd
}

// newDrainTenantCommand represents the drain command.
func newDrainTenantCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := drain.NewOptions(f)

	var cmd = &cobra.Command{
		Use:               "tenant",
		Short:             "Drain a tenant cluster",
		Long:              WithTemplate(liqoctlDrainTenantLongHelp),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.Tenants(ctx, f, 1),

		PreRun: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.Printer.AskConfirm("drain", options.SkipConfirm))
		},

		Run: func(_ *cobra.Command, args []string) {
			options.Name = args[0]
			output.ExitOnErr(options.RunDrainTenant(ctx))
		},
	}

	options.Factory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for drain completion")

	return cmd
}
