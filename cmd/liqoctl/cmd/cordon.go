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
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/cordon"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

const liqoctlCordonTenantLongHelp = `Cordon a tenant cluster.

This command allows to cordon a tenant cluster, preventing it from receiving new resources.
Resources provided by existing ResourceSlices are left untouched, while new ResourceSlices
are denied.

Examples:
  $ {{ .Executable }} cordon tenant my-tenant-name
`

const liqoctlCordonResourceSliceLongHelp = `Cordon a ResourceSlice.

This command allows to cordon a ResourceSlice, preventing it from receiving new resources.
Resources provided by existing ResourceSlices are left untouched, while new ones are denied.

Examples:
  $ {{ .Executable }} cordon resourceslice my-rs-name
`

// newCordonCommand represents the cordon command.
func newCordonCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "cordon",
		Short: "Cordon a liqo resource",
		Long:  "Cordon a liqo resource",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newCordonTenantCommand(ctx, f))
	cmd.AddCommand(newCordonResourceSliceCommand(ctx, f))

	return cmd
}

// newCordonTenantCommand represents the cordon command.
func newCordonTenantCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := cordon.NewOptions(f)

	var cmd = &cobra.Command{
		Use:               "tenant",
		Aliases:           []string{"tenants"},
		Short:             "Cordon a tenant cluster",
		Long:              WithTemplate(liqoctlCordonTenantLongHelp),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.Tenants(ctx, f, 1),

		PreRun: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.Printer.AskConfirm("cordon", options.SkipConfirm))
		},

		Run: func(_ *cobra.Command, args []string) {
			options.Name = args[0]
			output.ExitOnErr(options.RunCordonTenant(ctx))
		},
	}

	options.Factory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for cordon completion")

	return cmd
}

// newCordonResourceSliceCommand represents the cordon command.
func newCordonResourceSliceCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := cordon.NewOptions(f)

	var cmd = &cobra.Command{
		Use:               "resourceslice",
		Aliases:           []string{"resourceslices", "rs"},
		Short:             "Cordon a ResourceSlice",
		Long:              WithTemplate(liqoctlCordonResourceSliceLongHelp),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ResourceSlices(ctx, f, 1),

		PreRun: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.Printer.AskConfirm("cordon", options.SkipConfirm))
		},

		Run: func(_ *cobra.Command, args []string) {
			options.Name = args[0]
			output.ExitOnErr(options.RunCordonResourceSlice(ctx))
		},
	}

	options.Factory.AddFlags(cmd.PersistentFlags(), cmd.RegisterFlagCompletionFunc)

	cmd.Flags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for cordon completion")
	cmd.Flags().Var(&options.ClusterID, "remote-cluster-id", "ClusterID of the ResourceSlice to cordon")

	runtime.Must(cmd.MarkFlagRequired("remote-cluster-id"))
	runtime.Must(cmd.RegisterFlagCompletionFunc("remote-cluster-id", completion.ClusterIDs(ctx, f, completion.NoLimit)))

	return cmd
}
