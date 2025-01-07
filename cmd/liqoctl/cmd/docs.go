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
	"github.com/spf13/pflag"
	"k8s.io/kubectl/pkg/cmd/util"

	"github.com/liqotech/liqo/pkg/liqoctl/docs"
)

func newDocsCommand(ctx context.Context) *cobra.Command {
	options := docs.Options{}
	cmd := &cobra.Command{
		Use:   "docs",
		Short: WithTemplate("Generate {{ .Executable }} documentation"),
		Long:  WithTemplate("Generate {{ .Executable }} documentation"),
		Args:  cobra.NoArgs,

		Run: func(cmd *cobra.Command, _ []string) {
			options.Root = cmd.Root()
			util.CheckErr(options.Run(ctx))
		},

		Hidden: true,
	}

	cmd.Flags().StringVar(&options.Destination, "dir", ".", "The output directory for the generated documentation")
	cmd.Flags().StringVar(&options.DocTypeString, "type", "markdown", "The output documentation format, among markdown and man")
	cmd.Flags().BoolVar(&options.GenerateHeaders, "generate-headers", false, "Enable standard headers generation for markdown files")

	// Hide all flags inherited from the root command.
	cmd.SetHelpFunc(func(c *cobra.Command, s []string) {
		c.InheritedFlags().VisitAll(func(flag *pflag.Flag) { flag.Hidden = true })
		c.Parent().HelpFunc()(c, s)
	})

	return cmd
}
