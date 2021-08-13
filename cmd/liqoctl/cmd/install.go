package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

// installCmd represents the generateInstall command.
func newInstallCommand(ctx context.Context) *cobra.Command {
	var installCmd = &cobra.Command{
		Use:   "install",
		Short: install.LiqoctlInstallShortHelp,
		Long:  install.LiqoctlInstallLongHelp,
		Run: func(cmd *cobra.Command, args []string) {
			install.HandleInstallCommand(ctx, cmd, args)
		},
	}

	installCmd.Flags().StringP("provider", "p", "kubeadm", "Select the cluster provider type")
	installCmd.Flags().IntP("timeout", "t", 600, "Configure the timeout for the installation process in seconds (default: 600)")
	installCmd.Flags().StringP("version", "", "", "Select the Liqo version (default: latest stable release)")

	for _, p := range providers {
		initFunc, ok := providerInitFunc[p]
		if !ok {
			klog.Fatalf("unknown provider: %v", p)
		}
		initFunc(installCmd.Flags())
	}
	return installCmd
}
