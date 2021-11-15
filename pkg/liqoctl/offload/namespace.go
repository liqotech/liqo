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

package offload

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/common"
	"github.com/liqotech/liqo/pkg/utils"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	logsutils "github.com/liqotech/liqo/pkg/utils/logs"
)

// HandleOffloadCommand implements the "offload namespace" command.
// It forges and createOrUpdate a namespaceOffloading resource for a given namespace according to the flag values.
func HandleOffloadCommand(ctx context.Context, command *cobra.Command, args []string) error {
	if !klog.V(4).Enabled() {
		klog.SetLogFilter(logsutils.LogFilter{})
	}

	config, err := common.GetLiqoctlRestConf()
	if err != nil {
		return err
	}

	k8sClient, err := client.New(config, client.Options{})
	if err != nil {
		return err
	}

	var acceptedClusterLabels argsutils.StringMap
	if err := acceptedClusterLabels.Set(command.Flag(AcceptedLabelsFlag).Value.String()); err != nil {
		return err
	}
	var deniedClusterLabels argsutils.StringMap
	if err := deniedClusterLabels.Set(command.Flag(DeniedLabelsFlag).Value.String()); err != nil {
		return err
	}

	nsOffloading := forgeNamespaceOffloading(command, args, acceptedClusterLabels, deniedClusterLabels)

	_, err = controllerutil.CreateOrUpdate(ctx, k8sClient, nsOffloading, func() error {
		nsOffloading.Spec.PodOffloadingStrategy = forgePodOffloadingStrategy(command)
		nsOffloading.Spec.NamespaceMappingStrategy = forgeNamespaceMappingStrategy(command)
		nsOffloading.Spec.ClusterSelector = forgeClusterSelector(acceptedClusterLabels, deniedClusterLabels)
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf(SuccessfulMessage, args[0], args[0], consts.DefaultNamespaceOffloadingName)

	return nil
}

func forgeNamespaceOffloading(command *cobra.Command, args []string,
	acceptedLabels, deniedLabels argsutils.StringMap) *offloadingv1alpha1.NamespaceOffloading {
	return &offloadingv1alpha1.NamespaceOffloading{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consts.DefaultNamespaceOffloadingName,
			Namespace: args[0],
		},
		Spec: offloadingv1alpha1.NamespaceOffloadingSpec{
			NamespaceMappingStrategy: forgeNamespaceMappingStrategy(command),
			PodOffloadingStrategy:    forgePodOffloadingStrategy(command),
			ClusterSelector:          forgeClusterSelector(acceptedLabels, deniedLabels),
		},
	}
}

func forgeClusterSelector(acceptedLabels, deniedLabels argsutils.StringMap) corev1.NodeSelector {
	var l []corev1.NodeSelectorRequirement

	for k, v := range acceptedLabels.StringMap {
		l = append(l, corev1.NodeSelectorRequirement{
			Key:      k,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{v},
		})
	}

	for k, v := range deniedLabels.StringMap {
		l = append(l, corev1.NodeSelectorRequirement{
			Key:      k,
			Operator: corev1.NodeSelectorOpNotIn,
			Values:   []string{v},
		})
	}

	nodeSelector := utils.MergeNodeSelector(&corev1.NodeSelector{
		NodeSelectorTerms: []corev1.NodeSelectorTerm{{
			MatchExpressions: []corev1.NodeSelectorRequirement{{
				Key:      consts.TypeLabel,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{consts.TypeNode},
			}}}}}, &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
		MatchExpressions: l}}})
	return nodeSelector
}

func forgePodOffloadingStrategy(command *cobra.Command) offloadingv1alpha1.PodOffloadingStrategyType {
	return offloadingv1alpha1.PodOffloadingStrategyType(command.Flag(PodOffloadingStrategyFlag).Value.String())
}

func forgeNamespaceMappingStrategy(command *cobra.Command) offloadingv1alpha1.NamespaceMappingStrategyType {
	return offloadingv1alpha1.NamespaceMappingStrategyType(command.Flag(NamespaceMappingStrategyFlag).Value.String())
}
