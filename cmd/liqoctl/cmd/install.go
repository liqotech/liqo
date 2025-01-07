// Copyright 2019-2025 The Liqo Authors
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
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/install"
	"github.com/liqotech/liqo/pkg/liqoctl/install/aks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/eks"
	"github.com/liqotech/liqo/pkg/liqoctl/install/generic"
	"github.com/liqotech/liqo/pkg/liqoctl/install/gke"
	"github.com/liqotech/liqo/pkg/liqoctl/install/k3s"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kind"
	"github.com/liqotech/liqo/pkg/liqoctl/install/kubeadm"
	"github.com/liqotech/liqo/pkg/liqoctl/install/openshift"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/version"
	"github.com/liqotech/liqo/pkg/utils/args"
	kernelversion "github.com/liqotech/liqo/pkg/utils/kernel/version"
)

const liqoctlInstallLongHelp = `Install/upgrade Liqo in the selected cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
cluster, appropriately configuring it based on the provided flags. Additional
default values can be overridden through the --values and or --set flag.
Alternatively, it can be configured to only output a pre-configured values file,
which can be further customized and used for a manual installation with Helm.

By default, the command installs the latest released version of Liqo, although
this behavior can be tuned through the appropriate flags. In case a development
version is selected, and a local chart path is not specified, the command
proceeds cloning the Liqo repository (or the specified fork) at that version,
and leverages the included Helm chart. This is useful to install unreleased
versions, and during the local testing process.

Instead of directly using this generic command, it is suggested to leverage the
subcommand corresponding to the type of the target cluster (on-premise
distribution or cloud provider), which automatically retrieves most parameters
based on the cluster configuration.

Examples:
  $ {{ .Executable }} install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
or (configure the cluster id and labels)
  $ {{ .Executable }} install --cluster-id engaged-weevil --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24 --cluster-labels region=europe,environment=staging
or (generate and output the values file, instead of performing the installation)
  $ {{ .Executable }} install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 --only-output-values
or (install a specific Liqo version)
  $ {{ .Executable }} install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 --version v0.4.0
or (install a development version, using the default Helm chart)
  $ {{ .Executable }} install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --version 2058543d90482baf6f839eb57cbf3a9e81e20abe
or (install a development version, using a local Helm chart)
  $ {{ .Executable }} install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --version 2058543d90482baf6f839eb57cbf3a9e81e20abe --local-chart-path ./liqo/deployments/liqo
or (install a development version, cloning the Helm chart from a fork)
  $ {{ .Executable }} install --pod-cidr 10.0.0.0/16 --service-cidr 10.1.0.0/16 \
      --version 2058543d90482baf6f839eb57cbf3a9e81e20abe --repo-url https://github.com/fork/liqo.git
`

const liqoctlInstallProviderLongHelp = `Install/upgrade Liqo in the selected %s cluster.

This command wraps the Helm command to install/upgrade Liqo in the selected
%s cluster, automatically retrieving most parameters based on the cluster
configuration.

Please, refer to the help of the root *{{ .Executable }} install* command for additional
information and examples concerning its behavior and the common flags.

%s
`

func newInstallCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := install.NewOptions(f, liqoctl)
	base := generic.New(options)
	clusterLabels := args.StringMap{StringMap: map[string]string{}}
	reservedSubnets := args.CIDRList{}

	defaultRepoURL := "https://github.com/liqotech/liqo"

	var clusterIDFlag args.ClusterIDFlags

	var cmd = &cobra.Command{
		Use:     "install",
		Aliases: []string{"upgrade"},
		Short:   "Install/upgrade Liqo in the selected cluster",
		Long:    WithTemplate(liqoctlInstallLongHelp),
		Args:    cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			singleClusterPersistentPreRun(cmd, f)

			if !options.DisableKernelVersionCheck {
				if err := kernelversion.CheckKernelVersionFromNodes(ctx, f.CRClient, &kernelversion.MinimumKernelVersion); err != nil {
					options.Printer.ExitWithMessage(fmt.Sprintf("%v, disable this check with --%s", err, "disable-kernel-version-check"))
				}
			}

			options.ClusterID = clusterIDFlag.GetClusterID()
			options.ClusterLabels = clusterLabels.StringMap
			options.ReservedSubnets = reservedSubnets.StringList.StringList

			switch {
			case options.RepoURL != defaultRepoURL && options.ChartPath != "":
				options.Printer.ExitWithMessage("Cannot specify both --repo-url and --local-chart-path at the same time")
			case options.ChartPath != "" && options.Version == "":
				options.Printer.ExitWithMessage("A version must be explicitly specified if the --local-chart-path flag is set")
			case options.RepoURL != defaultRepoURL && options.Version == "":
				options.Printer.ExitWithMessage("A version must be explicitly specified if the --repo-url flag is set")
			}
		},

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.Run(ctx, base))
		},
	}

	cmd.PersistentFlags().StringVar(&options.Version, "version", version.LiqoctlVersion,
		"The version of Liqo to be installed, among releases and commit SHAs. Defaults to the liqoctl version")
	cmd.PersistentFlags().StringVar(&options.RepoURL, "repo-url", defaultRepoURL,
		"The URL of the git repository used to retrieve the Helm chart, if a non released version is specified")
	cmd.PersistentFlags().StringVar(&options.ChartPath, "local-chart-path", "",
		"The local path used to retrieve the Helm chart, instead of the upstream one")

	cmd.PersistentFlags().BoolVar(&options.OnlyOutputValues, "only-output-values", false,
		"Generate the pre-configured values file for further customization, instead of installing Liqo (default false)")
	// the default value is set during the validation to check if the flag has been set or not
	cmd.PersistentFlags().StringVar(&options.ValuesPath, "dump-values-path", "",
		"The path where the generated values file is saved (only in case --only-output-values is set). Default: './values.yaml'")
	cmd.PersistentFlags().BoolVar(&options.DryRun, "dry-run", false, "Simulate the installation process (default false)")

	cmd.PersistentFlags().DurationVar(&options.Timeout, "timeout", 10*time.Minute,
		"The timeout for the completion of the installation process")

	cmd.PersistentFlags().Var(&clusterIDFlag, "cluster-id", "The id identifying the cluster in Liqo")
	cmd.PersistentFlags().Var(&clusterLabels, "cluster-labels",
		"The set of labels (i.e., key/value pairs, separated by comma) identifying the current cluster, and propagated to the virtual nodes")

	cmd.PersistentFlags().Var(&reservedSubnets, "reserved-subnets",
		"The private CIDRs to be excluded, as already in use (e.g., the subnet of the cluster nodes); PodCIDR and ServiceCIDR shall not be included.")

	// Using StringArray rather than StringSlice: splitting is left to the Helm library, which takes care of special cases (e.g., lists).
	cmd.PersistentFlags().StringArrayVar(&options.OverrideValues, "set", []string{},
		"Set additional values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)")
	cmd.PersistentFlags().StringArrayVar(&options.OverrideStringValues, "set-string", []string{},
		"Set additional string values on the command line (can specify multiple times or separate values with commas: key1=val1,key2=val2)")
	cmd.PersistentFlags().StringArrayVar(&options.OverrideValuesFiles, "values", []string{},
		"Specify values in a YAML file or a URL (can specify multiple)")
	cmd.PersistentFlags().BoolVar(&options.DisableAPIServerSanityChecks, "disable-api-server-sanity-check", false,
		"Disable the sanity checks concerning the retrieved Kubernetes API server URL (default false)")
	cmd.PersistentFlags().BoolVar(&options.SkipValidation, "skip-validation", false, "Skip the validation of the arguments "+
		"(PodCIDR, ServiceCIDR). "+
		"This is useful when you are sure of what you are doing and the amount of pods and services in your cluster is very large (default false)")
	cmd.PersistentFlags().BoolVar(&options.EnableMetrics, "enable-metrics", false, "Enable metrics exposition through prometheus (default false)")
	cmd.PersistentFlags().BoolVar(&options.DisableTelemetry, "disable-telemetry", false,
		"Disable the anonymous and aggregated Liqo telemetry collection (default false)")
	cmd.PersistentFlags().BoolVar(&options.DisableKernelVersionCheck, "disable-kernel-version-check", false,
		"Disable the check of the minimum kernel version required to run the wireguard interface (default false)")

	f.AddLiqoNamespaceFlag(cmd.PersistentFlags())

	base.RegisterFlags(cmd)
	cmd.AddCommand(newInstallProviderCommand(ctx, options.CommonOptions, aks.New))
	cmd.AddCommand(newInstallProviderCommand(ctx, options.CommonOptions, eks.New))
	cmd.AddCommand(newInstallProviderCommand(ctx, options.CommonOptions, gke.New))
	cmd.AddCommand(newInstallProviderCommand(ctx, options.CommonOptions, k3s.New))
	cmd.AddCommand(newInstallProviderCommand(ctx, options.CommonOptions, kind.New))
	cmd.AddCommand(newInstallProviderCommand(ctx, options.CommonOptions, kubeadm.New))
	cmd.AddCommand(newInstallProviderCommand(ctx, options.CommonOptions, openshift.New))

	return cmd
}

func newInstallProviderCommand(ctx context.Context, commonOpts *install.CommonOptions,
	creator func(*install.Options) install.Provider) *cobra.Command {
	options := install.Options{CommonOptions: commonOpts}
	provider := creator(&options)

	cmd := &cobra.Command{
		Use:   provider.Name(),
		Short: fmt.Sprintf("Install Liqo in the selected %s cluster", provider.Name()),
		Long:  WithTemplate(fmt.Sprintf(liqoctlInstallProviderLongHelp, provider.Name(), provider.Name(), provider.Examples())),
		Args:  cobra.NoArgs,

		Run: func(_ *cobra.Command, _ []string) {
			output.ExitOnErr(options.Run(ctx, provider))
		},
	}

	provider.RegisterFlags(cmd)
	return cmd
}
