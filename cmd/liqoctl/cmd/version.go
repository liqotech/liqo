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

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/version"
)

const liqoctlVersionLongHelp = `Print the liqo CLI version and the deployed Liqo version.

The CLI version is embedded in the binary and directly displayed. The deployed
Liqo version version is determined based on the installed chart version.

Examples:
  $ {{ .Executable }} version
`

func newVersionCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := version.Options{Factory: f}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the liqo CLI version and the deployed Liqo version",
		Long:  WithTemplate(liqoctlVersionLongHelp),
		Args:  cobra.NoArgs,

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.Run(ctx))
		},
	}

	cmd.Flags().BoolVar(&options.ClientOnly, "client", false, "Show client version only (no server required) (default false)")

	f.AddLiqoNamespaceFlag(cmd.Flags())
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc(factory.FlagNamespace, completion.Namespaces(ctx, f, completion.NoLimit)))

	return cmd
}
