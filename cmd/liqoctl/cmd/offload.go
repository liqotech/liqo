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
	"time"

	"github.com/spf13/cobra"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/offload"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/args"
)

const liqoctlOffloadNamespaceLongHelp = `Offload a namespace to remote clusters.

Once a given namespace is selected for offloading, Liqo extends it across the
cluster boundaries, through the the automatic creation of twin namespaces in the
subset of selected remote clusters. Remote namespaces host the actual pods
offloaded in the corresponding cluster, as well as the additional resources
(i.e., Services, EndpointSlices, Ingresses, ConfigMaps, Secrets, PVCs and PVs)
propagated by the resource reflection process.

Namespace offloading can be tuned in terms of:
* Clusters: select the target clusters through virtual node labels.
* Pod offloading: whether pods should be scheduled on physical nodes only,
  virtual nodes only, or both. Forcing all pods to be scheduled locally enables
  the consumption of services from remote clusters.
* Naming: whether remote namespaces have the same name or a suffix is added to
  prevent conflicts.

Besides the direct offloading of a namespace, this command also provides the
possibility to generate and output the underlying NamespaceOffloading
resource, that can later be applied through automation tools.

Examples:
  $ {{ .Executable }} offload namespace foo
or
  $ {{ .Executable }} offload namespace foo --pod-offloading-strategy Remote --namespace-mapping-strategy EnforceSameName
or (cluster labels in logical AND)
  $ {{ .Executable }} offload namespace foo --namespace-mapping-strategy EnforceSameName \
      --selector 'region in (europe,us-west), !staging'
or (cluster labels in logical OR)
  $ {{ .Executable }} offload namespace foo --namespace-mapping-strategy EnforceSameName \
      --selector 'region in (europe,us-west)' --selector '!staging'
or (output the NamespaceOffloading resource as a yaml manifest, without applying it)
  $ {{ .Executable }} offload namespace foo --output yaml
`

func newOffloadCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "offload",
		Short: "Offload a resource to remote clusters",
		Long:  "Offload a resource to remote clusters.",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newOffloadNamespaceCommand(ctx, f))
	return cmd
}

func newOffloadNamespaceCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	var selectors []string

	podOffloadingStrategy := args.NewEnum([]string{
		string(offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType),
		string(offloadingv1beta1.RemotePodOffloadingStrategyType),
		string(offloadingv1beta1.LocalPodOffloadingStrategyType)},
		string(offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType))

	namespaceMappingStrategy := args.NewEnum([]string{
		string(offloadingv1beta1.EnforceSameNameMappingStrategyType),
		string(offloadingv1beta1.DefaultNameMappingStrategyType),
		string(offloadingv1beta1.SelectedNameMappingStrategyType)},
		string(offloadingv1beta1.DefaultNameMappingStrategyType))

	var remoteNamespaceName = ""

	outputFormat := args.NewEnum([]string{"json", "yaml"}, "")

	options := offload.Options{Factory: f}
	cmd := &cobra.Command{
		Use:     "namespace name",
		Aliases: []string{"ns"},
		Short:   "Offload a namespace to remote clusters",
		Long:    WithTemplate(liqoctlOffloadNamespaceLongHelp),

		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.Namespaces(ctx, f, 1),

		PreRun: func(_ *cobra.Command, _ []string) {
			options.PodOffloadingStrategy = offloadingv1beta1.PodOffloadingStrategyType(podOffloadingStrategy.Value)
			options.NamespaceMappingStrategy = offloadingv1beta1.NamespaceMappingStrategyType(namespaceMappingStrategy.Value)
			options.RemoteNamespaceName = remoteNamespaceName
			options.OutputFormat = outputFormat.Value
			options.Printer.CheckErr(options.ParseClusterSelectors(selectors))
		},

		Run: func(_ *cobra.Command, args []string) {
			options.Namespace = args[0]
			output.ExitOnErr(options.Run(ctx))
		},
	}

	cmd.Flags().Var(podOffloadingStrategy, "pod-offloading-strategy",
		"The constraints regarding pods scheduling in this namespace, among Local, Remote and LocalAndRemote")
	cmd.Flags().Var(namespaceMappingStrategy, "namespace-mapping-strategy",
		"The naming strategy adopted for the creation of remote namespaces, among DefaultName, EnforceSameName and SelectedName")
	cmd.Flags().DurationVar(&options.Timeout, "timeout", 20*time.Second, "The timeout for the offloading process")
	cmd.Flags().StringVar(&remoteNamespaceName, "remote-namespace-name", "",
		"The name of the remote namespace, required when using the SelectedName NamespaceMappingStrategy. "+
			"Otherwise, it is ignored")

	cmd.Flags().StringArrayVarP(&selectors, "selector", "l", []string{},
		"The selector to filter the target clusters. Can be specified multiple times, defining alternative requirements (i.e., in logical OR)")

	cmd.Flags().VarP(outputFormat, "output", "o",
		"Output the resulting NamespaceOffloading resource, instead of applying it. Supported formats: json, yaml")

	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("pod-offloading-strategy", completion.Enumeration(podOffloadingStrategy.Allowed)))
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("namespace-mapping-strategy", completion.Enumeration(namespaceMappingStrategy.Allowed)))
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("selector", completion.LabelsSelector(ctx, f, completion.NoLimit)))
	f.Printer.CheckErr(cmd.RegisterFlagCompletionFunc("output", completion.Enumeration(outputFormat.Allowed)))

	return cmd
}
