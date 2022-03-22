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
	"github.com/liqotech/liqo/pkg/liqoctl/autocompletion"
	"github.com/liqotech/liqo/pkg/liqoctl/offload"
	"github.com/liqotech/liqo/pkg/utils/args"
)

func newOffloadCommand(ctx context.Context) *cobra.Command {
	var offloadCommand = &cobra.Command{
		Use:          offload.UseCommand,
		SilenceUsage: true,
		Short:        offload.LiqoctlOffloadShortHelp,
		Long:         offload.LiqoctlOffloadLongHelp,
	}
	offloadCommand.AddCommand(newNamespaceCommand(ctx))
	return offloadCommand
}

func newNamespaceCommand(ctx context.Context) *cobra.Command {
	var offloadClusterCmd = &cobra.Command{
		Use:          offload.ClusterResourceName,
		Aliases:      []string{"ns"},
		SilenceUsage: true,
		Short:        offload.LiqoctlOffloadShortHelp,
		Long:         offload.LiqoctlOffloadLongHelp,
		Args:         cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return offload.HandleOffloadCommand(ctx, cmd, args)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) >= 1 {
				return nil, cobra.ShellCompDirectiveDefault
			}

			names, err := autocompletion.GetNamespaceNames(cmd.Context(), toComplete)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
	}

	podStrategies := []string{string(offloadingv1alpha1.LocalAndRemotePodOffloadingStrategyType),
		string(offloadingv1alpha1.RemotePodOffloadingStrategyType),
		string(offloadingv1alpha1.LocalPodOffloadingStrategyType)}
	nsStrategies := []string{string(offloadingv1alpha1.EnforceSameNameMappingStrategyType),
		string(offloadingv1alpha1.DefaultNameMappingStrategyType)}

	podOffloadingStrategy := args.NewEnum(podStrategies, string(offloadingv1alpha1.LocalAndRemotePodOffloadingStrategyType))
	offloadClusterCmd.PersistentFlags().Var(podOffloadingStrategy, offload.PodOffloadingStrategyFlag, offload.PodOffloadingStrategyHelp)

	namespaceMappingStrategy := args.NewEnum(nsStrategies, string(offloadingv1alpha1.DefaultNameMappingStrategyType))
	offloadClusterCmd.PersistentFlags().Var(namespaceMappingStrategy,
		offload.NamespaceMappingStrategyFlag, offload.NamespaceMappingStrategyHelp)

	offloadClusterCmd.PersistentFlags().String(offload.AcceptedLabelsFlag,
		offload.AcceptedLabelsDefault, offload.AcceptedLabelsHelp)
	offloadClusterCmd.PersistentFlags().String(offload.DeniedLabelsFlag,
		offload.DeniedLabelDefault, offload.DeniedLabelsHelp)

	utilruntime.Must(offloadClusterCmd.RegisterFlagCompletionFunc(offload.PodOffloadingStrategyFlag,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return podStrategies, cobra.ShellCompDirectiveNoFileComp
		}))
	utilruntime.Must(offloadClusterCmd.RegisterFlagCompletionFunc(offload.NamespaceMappingStrategyFlag,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return nsStrategies, cobra.ShellCompDirectiveNoFileComp
		}))

	return offloadClusterCmd
}
