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
	"flag"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// NewRootCommand initializes the tree of commands.
func NewRootCommand(ctx context.Context) *cobra.Command {
	// rootCmd represents the base command when called without any subcommands.
	var rootCmd = &cobra.Command{
		Use:   "liqoctl",
		Short: common.LiqoctlShortHelp,
		Long:  common.LiqoctlLongHelp,
	}

	// since we cannot access internal klog configuration, we create a new flagset, let klog to install
	// its flags, and we only keep the ones we are intrested in.
	klogFlagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(klogFlagset)
	klogFlagset.VisitAll(func(f *flag.Flag) {
		if f.Name == "v" {
			rootCmd.PersistentFlags().AddGoFlag(f)
		}
	})

	rateFlagset := flag.NewFlagSet("rate-limiting", flag.PanicOnError)
	restcfg.InitFlags(rateFlagset)
	rootCmd.PersistentFlags().AddGoFlagSet(rateFlagset)
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable/Disable debug mode (default: false)")
	rootCmd.AddCommand(newInstallCommand(ctx))
	rootCmd.AddCommand(newUninstallCommand(ctx))
	rootCmd.AddCommand(newAddCommand(ctx))
	rootCmd.AddCommand(newRemoveCommand(ctx))
	rootCmd.AddCommand(newGenerateAddCommand(ctx))
	rootCmd.AddCommand(newDocsCommand(ctx))
	rootCmd.AddCommand(newVersionCommand())
	rootCmd.AddCommand(newStatusCommand(ctx))
	rootCmd.AddCommand(newOffloadCommand(ctx))
	rootCmd.AddCommand(newConnectCommand(ctx))
	rootCmd.AddCommand(newDisconnectCommand(ctx))
	rootCmd.AddCommand(newMoveCommand(ctx))
	return rootCmd
}
