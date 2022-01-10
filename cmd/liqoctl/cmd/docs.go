// Copyright 2019-2022 The Liqo Authors
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
	"helm.sh/helm/v3/cmd/helm/require"

	"github.com/liqotech/liqo/pkg/liqoctl/docs"
)

func newDocsCommand(ctx context.Context) *cobra.Command {
	docsArgs := docs.Args{}
	cmd := &cobra.Command{
		Use:   docs.UseCommand,
		Short: docs.ShortHelp,
		Long:  docs.LongHelp,
		Args:  require.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			docsArgs.TopCmd = cmd.Root()
			return docsArgs.Handler(ctx)
		},
		Hidden: true,
	}
	flags := cmd.Flags()
	flags.StringVar(&docsArgs.Dest, docs.OutputDir, "./", "directory to which documentation is written")
	flags.StringVar(&docsArgs.DocTypeString, docs.DocType, "markdown", "the type of documentation to generate (markdown, man)")
	flags.BoolVar(&docsArgs.GenerateHeaders, docs.GenerateHeaders, false, "generate standard headers for markdown files")
	return cmd
}
