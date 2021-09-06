package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// installCmd represents the generateInstall command.
func newInstallCommand(ctx context.Context) *cobra.Command {
	var installCmd = &cobra.Command{
		Use:   installutils.LiqoctlInstallCommand,
		Short: installutils.LiqoctlInstallShortHelp,
		Long:  installutils.LiqoctlInstallLongHelp,
	}

	installCmd.PersistentFlags().IntP("timeout", "t", 600, "Configure the timeout for the installation process in seconds")
	installCmd.PersistentFlags().StringP("version", "", "", "Select the Liqo version (default: latest stable release)")
	installCmd.PersistentFlags().BoolP("devel", "", false,
		"Enable use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
	installCmd.PersistentFlags().BoolP("only-output-values", "", false, "Generate a values file for further customization")
	installCmd.PersistentFlags().StringP("dump-values-path", "", "./values.yaml", "Path for the output value file")
	installCmd.PersistentFlags().BoolP("dry-run", "", false, "Simulate an install")
	installCmd.PersistentFlags().BoolP("enable-lan-discovery", "", true, "Enable LAN discovery")
	installCmd.PersistentFlags().StringP("cluster-labels", "", "",
		"Cluster Labels to append to Liqo Cluster, supports '='.(e.g. --cluster-labels key1=value1,key2=value2)")
	installCmd.PersistentFlags().BoolP("disable-endpoint-check", "", false,
		"Disable the check that the current kubeconfig context contains the same endpoint retrieved from the cloud provider (AKS, EKS, GKE)")
	installCmd.PersistentFlags().String("chart-path", installutils.LiqoChartFullName,
		"Specify a path to get the Liqo chart, instead of installing the chart from the official repository")

	for _, providerName := range providers {
		cmd, err := getCommand(ctx, providerName)
		if err != nil {
			klog.Fatal(err)
		}

		installCmd.AddCommand(cmd)
	}
	return installCmd
}
