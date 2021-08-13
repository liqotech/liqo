package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/common"
)

// NewRootCommand initializes the tree of commands.
func NewRootCommand(ctx context.Context) *cobra.Command {
	// rootCmd represents the base command when called without any subcommands.
	var rootCmd = &cobra.Command{
		Use:   "liqoctl",
		Short: common.LiqoctlShortHelp,
		Long:  common.LiqoctlShortHelp,
	}
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable/Disable debug mode (default: false)")
	rootCmd.AddCommand(newInstallCommand(ctx))
	return rootCmd
}
