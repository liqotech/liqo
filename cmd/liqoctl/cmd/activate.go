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

	"github.com/liqotech/liqo/pkg/liqoctl/activate"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

const liqoctlActivateTenantLongHelp = `Activate a tenant cluster.

This command allows to activate a tenant cluster, allowing it to receive new resources.
Resources provided by existing ResourceSlices are provided again.

Examples:
  $ {{ .Executable }} activate tenant my-tenant-name
`

// newActivateCommand represents the activate command.
func newActivateCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "activate",
		Short: "Activate a liqo resource",
		Long:  "Activate a liqo resource",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newActivateTenantCommand(ctx, f))

	return cmd
}

// newActivateTenantCommand represents the activate command.
func newActivateTenantCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := activate.NewOptions(f)

	var cmd = &cobra.Command{
		Use:               "tenant",
		Short:             "Activate a tenant cluster",
		Long:              WithTemplate(liqoctlActivateTenantLongHelp),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.Tenants(ctx, f, 1),

		Run: func(_ *cobra.Command, args []string) {
			options.Name = args[0]
			output.ExitOnErr(options.RunActivateTenant(ctx))
		},
	}

	options.Factory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for activate completion")

	return cmd
}
