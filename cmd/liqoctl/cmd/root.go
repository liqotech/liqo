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
	"bytes"
	"context"
	"flag"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
	logsutils "github.com/liqotech/liqo/pkg/utils/logs"
	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// NewRootCommand initializes the tree of commands.
func NewRootCommand(ctx context.Context) *cobra.Command {
	// rootCmd represents the base command when called without any subcommands.
	var rootCmd = &cobra.Command{
		Use:   "liqoctl",
		Short: common.LiqoctlShortHelp,
		Long:  common.LiqoctlLongHelp,

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			verbose, err := cmd.Flags().GetBool("verbose")
			utilruntime.Must(err)

			printer := common.NewPrinter("", common.Cluster1Color)
			logsutils.SetupLogger(printer, verbose)
		},
	}

	// since we cannot access internal klog configuration, we create a new flagset, let klog to install
	// its flags, and we only set the ones we are intrested in.
	klogFlagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(klogFlagset)
	klogFlagset.VisitAll(func(f *flag.Flag) {
		// this is required to silence the helm library messages.
		if f.Name == "stderrthreshold" {
			utilruntime.Must(f.Value.Set("FATAL"))
		}
	})

	// this is required to silence the helm library messages.
	klog.LogToStderr(false)
	buffer := &bytes.Buffer{}
	klog.SetOutput(buffer)

	rateFlagset := flag.NewFlagSet("rate-limiting", flag.PanicOnError)
	restcfg.InitFlags(rateFlagset)
	rootCmd.PersistentFlags().AddGoFlagSet(rateFlagset)
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable/Disable verbose mode (default: false)")

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
