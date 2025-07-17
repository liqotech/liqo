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

package unoffload

import (
	"context"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
)

// Options encapsulates the arguments of the offload namespace command.
type Options struct {
	*factory.Factory

	Namespaces    []string
	LabelSelector string

	Timeout time.Duration
}

// Run implements the offload namespace command.
func (o *Options) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	var unoffloadNamespaces []string
	unoffloadNamespaces = append(unoffloadNamespaces, o.Namespaces...)

	if o.LabelSelector != "" {
		// Parse the label selector
		selector, err := labels.Parse(o.LabelSelector)
		if err != nil {
			return fmt.Errorf("invalid label selector: %w", err)
		}

		// List namespaces
		var selectedNsList corev1.NamespaceList
		if err := o.CRClient.List(ctx, &selectedNsList, &client.ListOptions{LabelSelector: selector}); err != nil {
			return fmt.Errorf("cannot list namespace objects: %w", err)
		}

		for i := range selectedNsList.Items {
			ns := &selectedNsList.Items[i]
			if !slices.Contains(unoffloadNamespaces, ns.Name) {
				unoffloadNamespaces = append(unoffloadNamespaces, ns.Name)
			}
		}
	}

	if len(unoffloadNamespaces) == 0 {
		o.Printer.Info.Println("No namespaces can be unoffloaded")
		return nil
	}

	var errors []error
	for _, ns := range unoffloadNamespaces {
		if err := o.runUnoffload(ctx, ns); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors during unoffloading, error details: %v", len(errors), errors)
	}

	return nil
}

func (o *Options) runUnoffload(ctx context.Context, namespace string) error {
	s := o.Printer.StartSpinner(fmt.Sprintf("Disabling namespace offloading for %q", namespace))
	nsoff := &offloadingv1beta1.NamespaceOffloading{ObjectMeta: metav1.ObjectMeta{
		Name: consts.DefaultNamespaceOffloadingName, Namespace: namespace}}
	if err := o.CRClient.Delete(ctx, nsoff); client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("Failed disabling namespace offloading: %v", err))
		return err
	}
	s.Success(fmt.Sprintf("Offloading of namespace %q correctly disabled", namespace))

	waiter := wait.NewWaiterFromFactory(o.Factory)
	if err := waiter.ForUnoffloading(ctx, namespace); err != nil {
		return err
	}

	return nil
}
