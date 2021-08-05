package cmd

import (
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "liqoctl",
	Short: "liqoctl - the Liqo Command Line Interface",
	Long: `liqoctl is a CLI tool to install and manage Liqo-enabled clusters.

Liqo is a platform to enable dynamic and decentralized resource sharing across Kubernetes clusters. 
Liqo allows to run pods on a remote cluster seamlessly and without any modification of 
Kubernetes and the applications. 
With Liqo it is possible to extend the control plane of a Kubernetes cluster across the cluster's boundaries, 
making multi-cluster native and transparent: collapse an entire remote cluster to a virtual local node, 
by allowing workloads offloading and resource management compliant with the standard Kubernetes approach.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}
