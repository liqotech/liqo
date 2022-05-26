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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
)

// Options encapsulates the arguments of the offload namespace command.
type Options struct {
	*factory.Factory

	Namespace                string
	PodOffloadingStrategy    offloadingv1alpha1.PodOffloadingStrategyType
	NamespaceMappingStrategy offloadingv1alpha1.NamespaceMappingStrategyType
	ClusterSelector          [][]metav1.LabelSelectorRequirement

	Timeout time.Duration
}

const successMessage = `
Check the offloading status with:
$ kubectl get namespaceoffloading -n %s %s
`

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

	s := o.Printer.StartSpinner(fmt.Sprintf("Enabling namespace offloading for %q", o.Namespace))

	nsoff := &offloadingv1alpha1.NamespaceOffloading{ObjectMeta: metav1.ObjectMeta{
		Name: consts.DefaultNamespaceOffloadingName, Namespace: o.Namespace}}

	_, err := controllerutil.CreateOrUpdate(ctx, o.CRClient, nsoff, func() error {
		nsoff.Spec.PodOffloadingStrategy = o.PodOffloadingStrategy
		nsoff.Spec.NamespaceMappingStrategy = o.NamespaceMappingStrategy
		nsoff.Spec.ClusterSelector = toNodeSelector(o.ClusterSelector)
		return nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed enabling namespace offloading: %v", err))
		return err
	}
	s.Success(fmt.Sprintf("Offloading of namespace %q correctly enabled", o.Namespace))

	waiter := wait.NewWaiterFromFactory(o.Factory)
	if err := waiter.ForOffloading(ctx, o.Namespace); err != nil {
		return err
	}

	fmt.Printf(successMessage, o.Namespace, consts.DefaultNamespaceOffloadingName)
	return nil
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
