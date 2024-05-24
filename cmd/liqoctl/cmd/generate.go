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

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
)

func newGenerateCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate data/commands to perform additional operations",
		Long:  "Generate data/commands to perform additional operations.",
		Args:  cobra.NoArgs,
	}

	options := &rest.GenerateOptions{
		Factory: f,
	}

	for _, r := range liqoResources {
		api := r()

		apiOptions := api.APIOptions()
		if apiOptions.EnableGenerate {
			cmd.AddCommand(api.Generate(ctx, options))
		}
	}

	f.AddNamespaceFlag(cmd.PersistentFlags())
	f.AddLiqoNamespaceFlag(cmd.PersistentFlags())

	return cmd
}
