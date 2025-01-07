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
	"github.com/liqotech/liqo/pkg/liqoctl/unoffload"
)

const liqoctlUnoffloadNamespaceLongHelp = `Unoffload a namespace from remote clusters.

This command disables the offloading of a namespace, deleting all resources
reflected to remote clusters (including the namespaces themselves), and causing
all offloaded pods to be rescheduled locally.

Examples:
  $ {{ .Executable }} unoffload namespace foo
`

func newUnoffloadCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unoffload",
		Short: "Unoffload a resource from remote clusters",
		Long:  "Unoffload a resource from remote clusters.",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newUnoffloadNamespaceCommand(ctx, f))
	return cmd
}

func newUnoffloadNamespaceCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := unoffload.Options{Factory: f}
	cmd := &cobra.Command{
		Use:     "namespace name",
		Aliases: []string{"ns"},
		Short:   "Unoffload a namespace from remote clusters",
		Long:    WithTemplate(liqoctlUnoffloadNamespaceLongHelp),

		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.OffloadedNamespaces(ctx, f, 1),

		PreRun: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(f.Printer.AskConfirm("unoffload", f.SkipConfirm))
		},

		Run: func(_ *cobra.Command, args []string) {
			options.Namespace = args[0]
			output.ExitOnErr(options.Run(ctx))
		},
	}

	cmd.Flags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for the unoffload operation")

	return cmd
}
