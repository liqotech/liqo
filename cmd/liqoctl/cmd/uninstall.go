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

	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
	"github.com/liqotech/liqo/pkg/liqoctl/uninstall"
)

const (
	// liqoctlUninstallShortHelp contains the short help message for uninstall Liqoctl command.
	liqoctlUninstallShortHelp = "Uninstall Liqo on a selected cluster"
	// liqoctlUninstallLongHelp contains the long help message for uninstall Liqoctl command.
	liqoctlUninstallLongHelp = `Uninstall Liqo on a selected cluster`

	// liqoctlUninstallPurgeHelp contains the help message for the purge flag.
	liqoctlUninstallPurgeHelp = "Purge all Liqo CRDs from the cluster"
	// liqoctlUninstallNamespaceHelp contains the help message for the namespace flag.
	liqoctlUninstallNamespaceHelp = "Namespace where Liqo is installed"
)

// newUninstallCommand generates a new Command representing `liqoctl uninstall`.
func newUninstallCommand(ctx context.Context) *cobra.Command {
	var uninstallArgs uninstall.Args

	var uninstallCmd = &cobra.Command{
		Use:          "uninstall",
		Short:        liqoctlUninstallShortHelp,
		Long:         liqoctlUninstallLongHelp,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return uninstall.HandleUninstallCommand(ctx, cmd, &uninstallArgs)
		},
	}

	uninstallCmd.Flags().StringVarP(&uninstallArgs.Namespace, "namespace", "n", installutils.LiqoNamespace, liqoctlUninstallNamespaceHelp)
	uninstallCmd.Flags().BoolVar(&uninstallArgs.Purge, "purge", false, liqoctlUninstallPurgeHelp)

	return uninstallCmd
}
