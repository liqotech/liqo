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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/completion"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/offload"
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
`

func newOffloadCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "offload",
		Short: "Offload a resource to remote clusters",
		Long:  "Offload a resource to remote clusters.",
		Args:  cobra.NoArgs,

		PersistentPreRun: func(cmd *cobra.Command, args []string) { singleClusterPersistentPreRun(cmd, f) },
	}

	cmd.AddCommand(newOffloadNamespaceCommand(ctx, f))
	return cmd
}

func newOffloadNamespaceCommand(ctx context.Context, f *factory.Factory) *cobra.Command {
	var selectors []string

	podOffloadingStrategy := args.NewEnum([]string{
		string(offloadingv1alpha1.LocalAndRemotePodOffloadingStrategyType),
		string(offloadingv1alpha1.RemotePodOffloadingStrategyType),
		string(offloadingv1alpha1.LocalPodOffloadingStrategyType)},
		string(offloadingv1alpha1.LocalAndRemotePodOffloadingStrategyType))

	namespaceMappingStrategy := args.NewEnum([]string{
		string(offloadingv1alpha1.EnforceSameNameMappingStrategyType),
		string(offloadingv1alpha1.DefaultNameMappingStrategyType)},
		string(offloadingv1alpha1.DefaultNameMappingStrategyType))

	options := offload.Options{Factory: f}
	cmd := &cobra.Command{
		Use:     "namespace name",
		Aliases: []string{"ns"},
		Short:   "Offload a namespace to remote clusters",
		Long:    WithTemplate(liqoctlOffloadNamespaceLongHelp),

		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.Namespaces(ctx, f, 1),

		PreRun: func(cmd *cobra.Command, args []string) {
			options.PodOffloadingStrategy = offloadingv1alpha1.PodOffloadingStrategyType(podOffloadingStrategy.Value)
			options.NamespaceMappingStrategy = offloadingv1alpha1.NamespaceMappingStrategyType(namespaceMappingStrategy.Value)
			options.Printer.CheckErr(options.ParseClusterSelectors(selectors))
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			options.Namespace = args[0]
			return options.Run(ctx)
		},
	}

	cmd.Flags().Var(podOffloadingStrategy, "pod-offloading-strategy",
		"The constraints regarding pods scheduling in this namespace, among Local, Remote and LocalAndRemote")
	cmd.Flags().Var(namespaceMappingStrategy, "namespace-mapping-strategy",
		"The naming strategy adopted for the creation of remote namespaces, among DefaultName and EnforceSameName")

	cmd.Flags().StringArrayVarP(&selectors, "selector", "l", []string{},
		"The selector to filter the target clusters. Can be specified multiple times, defining alternative requirements (i.e., in logical OR)")

	utilruntime.Must(cmd.RegisterFlagCompletionFunc("pod-offloading-strategy", completion.Enumeration(podOffloadingStrategy.Allowed)))
	utilruntime.Must(cmd.RegisterFlagCompletionFunc("namespace-mapping-strategy", completion.Enumeration(namespaceMappingStrategy.Allowed)))

	return cmd
}
