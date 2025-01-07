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

package offload

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// Options encapsulates the arguments of the offload namespace command.
type Options struct {
	*factory.Factory

	Namespace                string
	PodOffloadingStrategy    offloadingv1beta1.PodOffloadingStrategyType
	NamespaceMappingStrategy offloadingv1beta1.NamespaceMappingStrategyType
	RemoteNamespaceName      string
	ClusterSelector          [][]metav1.LabelSelectorRequirement

	OutputFormat string

	Timeout time.Duration
}

// ParseClusterSelectors parses the cluster selector.
func (o *Options) ParseClusterSelectors(selectors []string) error {
	for _, selector := range selectors {
		s, err := metav1.ParseToLabelSelector(selector)
		if err != nil {
			return err
		}

		// Convert MatchLabels into MatchExpressions
		for key, value := range s.MatchLabels {
			req := metav1.LabelSelectorRequirement{Key: key, Operator: metav1.LabelSelectorOpIn, Values: []string{value}}
			s.MatchExpressions = append(s.MatchExpressions, req)
		}

		o.ClusterSelector = append(o.ClusterSelector, s.MatchExpressions)
	}

	return nil
}

// Run implements the offload namespace command.
func (o *Options) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	// Output the NamespaceOffloading resource, instead of applying it.
	if o.OutputFormat != "" {
		o.Printer.CheckErr(o.output())
		return nil
	}

	s := o.Printer.StartSpinner(fmt.Sprintf("Enabling namespace offloading for %q", o.Namespace))

	nsoff := &offloadingv1beta1.NamespaceOffloading{ObjectMeta: metav1.ObjectMeta{
		Name: consts.DefaultNamespaceOffloadingName, Namespace: o.Namespace}}

	var oldStrategy offloadingv1beta1.PodOffloadingStrategyType
	_, err := resource.CreateOrUpdate(ctx, o.CRClient, nsoff, func() error {
		oldStrategy = nsoff.Spec.PodOffloadingStrategy
		nsoff.Spec.PodOffloadingStrategy = o.PodOffloadingStrategy
		nsoff.Spec.NamespaceMappingStrategy = o.NamespaceMappingStrategy
		nsoff.Spec.RemoteNamespaceName = o.RemoteNamespaceName
		nsoff.Spec.ClusterSelector = toNodeSelector(o.ClusterSelector)
		return nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed enabling namespace offloading: %v", output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Offloading of namespace %q correctly enabled", o.Namespace))

	switch {
	case oldStrategy == "", // The NamespaceOffloading has just been created
		o.PodOffloadingStrategy == oldStrategy,                                               // The pod offloading strategy has not been changed
		o.PodOffloadingStrategy == offloadingv1beta1.LocalAndRemotePodOffloadingStrategyType: // The new pod offloading strategy is less restrictive
		break
	default:
		o.Printer.Warning.Println("The PodOffloadingStrategy was mutated to a more restrictive setting")
		o.Printer.Warning.Println("Existing pods violating this policy might still be running")
	}

	waiter := wait.NewWaiterFromFactory(o.Factory)
	if err := waiter.ForOffloading(ctx, o.Namespace); err != nil {
		return err
	}

	return nil
}

// output implements the logic to output the generated NamespaceOffloading resource.
func (o *Options) output() error {
	var printer printers.ResourcePrinter
	switch o.OutputFormat {
	case "yaml":
		printer = &printers.YAMLPrinter{}
	case "json":
		printer = &printers.JSONPrinter{}
	default:
		return fmt.Errorf("unsupported output format %q", o.OutputFormat)
	}

	nsoff := offloadingv1beta1.NamespaceOffloading{
		TypeMeta:   metav1.TypeMeta{APIVersion: offloadingv1beta1.SchemeGroupVersion.String(), Kind: "NamespaceOffloading"},
		ObjectMeta: metav1.ObjectMeta{Name: consts.DefaultNamespaceOffloadingName, Namespace: o.Namespace},
		Spec: offloadingv1beta1.NamespaceOffloadingSpec{
			PodOffloadingStrategy:    o.PodOffloadingStrategy,
			NamespaceMappingStrategy: o.NamespaceMappingStrategy,
			RemoteNamespaceName:      o.RemoteNamespaceName,
			ClusterSelector:          toNodeSelector(o.ClusterSelector),
		},
	}

	return printer.PrintObj(&nsoff, os.Stdout)
}

func toNodeSelector(selectors [][]metav1.LabelSelectorRequirement) corev1.NodeSelector {
	terms := []corev1.NodeSelectorTerm{}

	for _, selector := range selectors {
		var requirements []corev1.NodeSelectorRequirement

		for _, r := range selector {
			requirements = append(requirements, corev1.NodeSelectorRequirement{
				Key:      r.Key,
				Operator: corev1.NodeSelectorOperator(r.Operator),
				Values:   r.Values,
			})
		}

		terms = append(terms, corev1.NodeSelectorTerm{MatchExpressions: requirements})
	}

	return corev1.NodeSelector{NodeSelectorTerms: terms}
}
