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
//

package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/info"
	"github.com/liqotech/liqo/pkg/liqoctl/info/localstatus"
	"github.com/liqotech/liqo/pkg/liqoctl/info/peer"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/args"
)

var outputFormat = args.NewEnum([]string{"json", "yaml"}, "")

const liqoctlInfoLongHelp = `Show info about the current Liqo instance.

Liqoctl provides a set of commands to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in human-readable or
machine-readable format (either JSON or YAML).
Additionally, via '--get', it allows to retrieve each single field of the reports
using a query in dot notation (e.g. '--get field.subfield')

This command shows information about the local cluster and checks the presence
and the sanity of the Liqo namespace and the Liqo pods and some brief info about
the active peerings and their status.

Examples:
  $ {{ .Executable }} info
  $ {{ .Executable }} info --namespace liqo-system
show the output in YAML format
  $ {{ .Executable }} info -o yaml
get a specific field
  $ {{ .Executable }} info --get clusterid
  $ {{ .Executable }} info --get network.podcidr
`

const liqoctlInfoPeerLongHelp = `Show additional info about peered clusters.

Liqoctl provides a set of commands to verify the status of the Liqo control
plane, its configuration, as well as the characteristics of the currently
active peerings, and reports the outcome in human-readable or
machine-readable format (either JSON or YAML).
Additionally, via '--get', it allows to retrieve each single field of the reports
using a query in dot notation (e.g. '--get field.subfield')

This command shows additional information about the peered clusters, the status
of the modules and the amount of shared resources.

Examples:
  $ {{ .Executable }} info peer
or
  $ {{ .Executable }} info peer cluster1
or
  $ {{ .Executable }} info peer cluster1 cluster2
or
  $ {{ .Executable }} info peer cluster1 cluster2 --namespace liqo-system
show the output in YAML format
  $ {{ .Executable }} info peer -o yaml
get a specific field
  $ {{ .Executable }} info peer cluster1 cluster2 --get cluster2.network.cidr
when a single cluster is specified, the cluster ID at the beginning of the query can be omitted
  $ {{ .Executable }} info peer cluster1 --get network.cidr
`

func infoPreRun(options *info.Options) {
	// When the output is redirected to a file is desiderable that errors ends in the stderr output.
	options.Printer.Error.Writer = os.Stderr
	options.Printer.Warning.Writer = os.Stderr
	// Configure output according to the provided parameter
	options.Format = info.OutputFormat(outputFormat.Value)
	// Force verbose when `get` is used to allow to retrieve also
	// the info in the verbose output also when "--verbose" is not provided
	if options.GetQuery != "" {
		options.Verbose = true
	}
}

func newPeerInfoCommand(ctx context.Context, f *factory.Factory, options *info.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "peer",
		Short:             "Show additional info about peered clusters",
		Long:              WithTemplate(liqoctlInfoPeerLongHelp),
		ValidArgsFunction: completion.ClusterIDs(ctx, f, completion.NoLimit),

		PreRun: func(_ *cobra.Command, _ []string) {
			infoPreRun(options)
		},

		Run: func(_ *cobra.Command, clusterIds []string) {
			checkers := []info.MultiClusterChecker{
				&peer.InfoChecker{},
				&peer.NetworkChecker{},
				&peer.AuthChecker{},
				&peer.OffloadingChecker{},
			}

			output.ExitOnErr(options.RunPeerInfo(ctx, checkers, clusterIds))
		},
	}

	return cmd
}

func newInfoCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	options := info.NewOptions(f)

	maincmd := &cobra.Command{
		Use:   "info",
		Short: "Show info about the current Liqo instance",
		Long:  WithTemplate(liqoctlInfoLongHelp),
		Args:  cobra.NoArgs,

		PreRun: func(_ *cobra.Command, _ []string) {
			infoPreRun(options)
		},

		Run: func(_ *cobra.Command, _ []string) {
			// Set up checkers
			checkers := []info.Checker{
				&localstatus.InstallationChecker{},
				&localstatus.HealthChecker{},
			}
			if options.Verbose {
				checkers = append(checkers, &localstatus.NetworkChecker{})
			}
			checkers = append(checkers, &localstatus.PeeringChecker{})
			output.ExitOnErr(options.RunInfo(ctx, checkers))
		},
	}

	f.AddLiqoNamespaceFlag(maincmd.PersistentFlags())
	maincmd.PersistentFlags().BoolVarP(&options.Verbose, "verbose", "v", false, "Make info more verbose")
	maincmd.PersistentFlags().VarP(outputFormat, "output", "o", "Output format. Supported formats: json, yaml")
	maincmd.PersistentFlags().StringVarP(&options.GetQuery, "get", "g", "",
		"Path to the desired subfield in dot notation. Each part of the path corresponds to a key of the output structure")

	f.Printer.CheckErr(maincmd.RegisterFlagCompletionFunc(factory.FlagNamespace, completion.Namespaces(ctx, f, completion.NoLimit)))
	f.Printer.CheckErr(maincmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))

	maincmd.AddCommand(newPeerInfoCommand(ctx, f, options))

	return maincmd
}
