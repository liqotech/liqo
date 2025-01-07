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

package create

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
)

// NewCreateCommand returns the cobra command for the create subcommand.
func NewCreateCommand(ctx context.Context, liqoResources []rest.APIProvider, f *factory.Factory) *cobra.Command {
	options := &rest.CreateOptions{
		Factory: f,
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create Liqo resources",
		Long:  "Create Liqo resources.",
		Args:  cobra.NoArgs,
	}

	f.AddNamespaceFlag(cmd.PersistentFlags())
	f.AddLiqoNamespaceFlag(cmd.PersistentFlags())

	for _, r := range liqoResources {
		api := r()

		apiOptions := api.APIOptions()
		if apiOptions.EnableCreate {
			cmd.AddCommand(api.Create(ctx, options))
		}
	}

	return cmd
}
