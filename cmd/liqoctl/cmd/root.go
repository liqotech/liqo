package cmd

import (
	"context"
	"flag"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
)

// NewRootCommand initializes the tree of commands.
func NewRootCommand(ctx context.Context) *cobra.Command {
	// rootCmd represents the base command when called without any subcommands.
	var rootCmd = &cobra.Command{
		Use:   "liqoctl",
		Short: common.LiqoctlShortHelp,
		Long:  common.LiqoctlLongHelp,
	}
	flagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(flagset)
	rootCmd.PersistentFlags().AddGoFlagSet(flagset)
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable/Disable debug mode (default: false)")
	rootCmd.AddCommand(newInstallCommand(ctx))
	return rootCmd
}
