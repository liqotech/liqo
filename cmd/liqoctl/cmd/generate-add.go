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
	var addCmd = &cobra.Command{
		Use:   generate.LiqoctlGenerateAddCommand,
		Short: generate.LiqoctlGenerateShortHelp,
		Long:  generate.LiqoctlGenerateLongHelp,
		Run: func(cmd *cobra.Command, args []string) {
			generate.HandleGenerateAddCommand(ctx, liqoNamespace, os.Args[0])
		},
	}
	addCmd.Flags().StringVar(&liqoNamespace, "namespace", add.ClusterLiqoNamespace,
		"the name of the namespace where Liqo is installed")

	return addCmd
}
