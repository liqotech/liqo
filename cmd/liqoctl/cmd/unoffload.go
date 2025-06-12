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
	"github.com/liqotech/liqo/pkg/liqoctl/utils"
)

const liqoctlUnoffloadNamespaceLongHelp = `Unoffload one or more namespaces from remote clusters.

This command disables the offloading of one or more namespaces, deleting all resources
reflected to remote clusters (including the namespaces themselves), and causing
all offloaded pods to be rescheduled locally.

Examples:
  $ {{ .Executable }} unoffload namespace foo
or
  $ {{ .Executable }} unoffload namespace foo bar
or
  $ {{ .Executable }} unoffload namespace --ns-selector 'foo=bar'
`

func newUnoffloadCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unoffload",
		Short: "Unoffload a resource from remote clusters",
		Long:  "Unoffload a resource from remote clusters.",
		Args:  cobra.NoArgs,
	}

	utils.AddCommand(cmd, newUnoffloadNamespaceCommand(ctx, f))
	return cmd
}

func newUnoffloadNamespaceCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	var labelSelector string

	options := unoffload.Options{Factory: f}
	cmd := &cobra.Command{
		Use:     "namespace name",
		Aliases: []string{"ns", "namespaces"},
		Short:   "Unoffload namespaces from remote clusters",
		Long:    liqoctlUnoffloadNamespaceLongHelp,

		ValidArgsFunction: completion.OffloadedNamespaces(ctx, f, completion.NoLimit),

		PreRun: func(_ *cobra.Command, _ []string) {
			options.LabelSelector = labelSelector
			output.ExitOnErr(f.Printer.AskConfirm("unoffload", f.SkipConfirm))
		},

		Run: func(_ *cobra.Command, args []string) {
			if len(args) == 0 && labelSelector == "" {
				options.Printer.ExitWithMessage("namespace name or label selector must be specified")
			}
			if len(args) != 0 && labelSelector != "" {
				options.Printer.ExitWithMessage("namespace name and label selector must not be specified together")
			}
			options.Namespaces = args
			output.ExitOnErr(options.Run(ctx))
		},
	}

	cmd.Flags().StringVar(&labelSelector, "ns-selector", "",
		"Selector (label query) to filter namespaces, supports '=', '==', and '!=' (e.g., -l key1=value1,key2=value2).")
	cmd.Flags().DurationVar(&options.Timeout, "timeout", 120*time.Second, "Timeout for the unoffload operation")

	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("ns-selector", completion.NamespacesSelector(ctx, f, completion.NoLimit)))

	return cmd
}
