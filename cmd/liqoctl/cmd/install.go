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
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
	installutils "github.com/liqotech/liqo/pkg/liqoctl/install/utils"
)

// newInstallCommand generates a new Command representing `liqoctl install`.
func newInstallCommand(ctx context.Context) *cobra.Command {
	var installCmd = &cobra.Command{
		Use:   installutils.LiqoctlInstallCommand,
		Short: installutils.LiqoctlInstallShortHelp,
		Long:  installutils.LiqoctlInstallLongHelp,
	}

	installCmd.PersistentFlags().IntP("timeout", "t", 600, "Configure the timeout for the installation process in seconds")
	installCmd.PersistentFlags().String("repo-url", "https://github.com/liqotech/liqo.git",
		"Configure the URL of the repository to use for the installation process")
	installCmd.PersistentFlags().StringP("version", "", "", "Select the Liqo version (default: latest stable release)")
	installCmd.PersistentFlags().BoolP("devel", "", false,
		"Enable use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored")
	installCmd.PersistentFlags().BoolP("only-output-values", "", false, "Generate a values file for further customization")
	installCmd.PersistentFlags().StringP("dump-values-path", "", "./values.yaml", "Path for the output value file")
	installCmd.PersistentFlags().BoolP("dry-run", "", false, "Simulate an install")
	installCmd.PersistentFlags().BoolP(liqoconst.EnableLanDiscoveryParameter, "", false, "Enable LAN discovery")
	installCmd.PersistentFlags().StringP(liqoconst.ClusterLabelsParameter, "", "",
		"Cluster Labels to append to Liqo Cluster, supports '='.(e.g. --cluster-labels key1=value1,key2=value2)")
	installCmd.PersistentFlags().BoolP("disable-endpoint-check", "", false,
		"Disable the check that the current kubeconfig context contains the same endpoint retrieved from the cloud provider (AKS, EKS, GKE)")
	installCmd.PersistentFlags().String("chart-path", installutils.LiqoChartFullName,
		"Specify a path to get the Liqo chart, instead of installing the chart from the official repository")
	installCmd.PersistentFlags().StringP(liqoconst.ClusterNameParameter, "n", "", "Name to assign to the Liqo cluster")
	installCmd.PersistentFlags().Bool(liqoconst.GenerateNameParameter, false, "Generate a random Docker-like name for the cluster")
	installCmd.PersistentFlags().String(liqoconst.ReservedSubnetsParameter, "", "In order to prevent IP conflicting between "+
		"locally used private subnets in your infrastructure and private subnets belonging to remote clusters "+
		"you need tell liqo the subnets used in your cluster. E.g if your cluster nodes belong to the 192.168.2.0/24 subnet then "+
		"you should add that subnet to the reservedSubnets. PodCIDR and serviceCIDR used in the local cluster are automatically "+
		"added to the reserved list. (e.g. --reserved-subnets 192.168.2.0/24,192.168.4.0/24)")
	installCmd.PersistentFlags().String("resource-sharing-percentage", "90", "It defines the percentage of available cluster resources that "+
		"you are willing to share with foreign clusters. It accepts [0 - 100] values.")
	installCmd.PersistentFlags().Bool("enable-ha", false, "Enable the gateway component support active/passive high availability.")
	installCmd.PersistentFlags().Int("mtu", 0, "mtu is the maximum transmission unit for interfaces managed by Liqo")
	installCmd.PersistentFlags().Int("vpn-listening-port", liqoconst.GatewayListeningPort, "vpn-listening-port is the port used by the vpn tunnel")

	for _, providerName := range provider.Providers {
		cmd, err := getCommand(ctx, providerName)
		if err != nil {
			klog.Fatal(err)
		}

		installCmd.AddCommand(cmd)
	}
	return installCmd
}
