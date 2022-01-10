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
	"os"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/add"
	"github.com/liqotech/liqo/pkg/liqoctl/generate"
)

// installCmd represents the generateInstall command.
func newGenerateAddCommand(ctx context.Context) *cobra.Command {
	var liqoNamespace string
	var onlyCommand bool
	var addCmd = &cobra.Command{
		Use:          generate.LiqoctlGenerateAddCommand,
		Short:        generate.LiqoctlGenerateShortHelp,
		Long:         generate.LiqoctlGenerateLongHelp,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generate.HandleGenerateAddCommand(ctx, liqoNamespace, onlyCommand, os.Args[0])
		},
	}
	addCmd.Flags().StringVar(&liqoNamespace, "namespace", add.ClusterLiqoNamespace,
		"the name of the namespace where Liqo is installed")
	addCmd.Flags().BoolVar(&onlyCommand, "only-command", false, "print only the add command (useful in scripts)")

	return addCmd
}
