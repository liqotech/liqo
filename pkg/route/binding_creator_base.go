// Copyright 2019-2026 The Liqo Authors
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

package route

import (
	"context"
	"fmt"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// OldRouteConfigurationFinalizer is the finalizer used by the legacy (pre-binding)
// route controller. It is no longer needed because cleanup is now handled per-binding,
// but it may still be present on RouteConfiguration resources created before the
// migration. The binding creator controllers automatically strip it.
const OldRouteConfigurationFinalizer = "routeconfiguration-controller.liqo.io/finalizer"

// BindingCreatorBase contains the shared logic for creating and managing
// RouteConfigurationBinding resources. It is embedded by the fabric and
// gateway specific binding creator reconcilers.
type BindingCreatorBase struct {
	client.Client
	Scheme *runtime.Scheme
}

// ReconcileBindings is the core orchestration method shared by both the fabric and
// gateway binding creator reconcilers. It strips the legacy finalizer and ensures a
// binding exists for each target.
func (b *BindingCreatorBase) ReconcileBindings(ctx context.Context,
	routecfg *networkingv1beta1.RouteConfiguration, targets []networkingv1beta1.RouteBindingTargetReference) error {
	// Remove the legacy finalizer that was managed by the old per-RouteConfiguration
	// controller. It is no longer needed because each binding now carries its own
	// finalizer, but clusters upgraded from the previous architecture may still have
	// it set. Stripping it here prevents the resource from being stuck during deletion.
	if controllerutil.ContainsFinalizer(routecfg, OldRouteConfigurationFinalizer) {
		original := routecfg.DeepCopy()
		controllerutil.RemoveFinalizer(routecfg, OldRouteConfigurationFinalizer)
		if err := b.Patch(ctx, routecfg, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("removing old finalizer from %s/%s: %w", routecfg.Namespace, routecfg.Name, err)
		}
		klog.Infof("Removed old finalizer from RouteConfiguration %s/%s", routecfg.Namespace, routecfg.Name)
	}

	// Ensure a binding exists for each target.
	for i := range targets {
		t := &targets[i]
		bindingName := BindingResourceName(routecfg.Name, t.Name)
		if err := b.ensureBinding(ctx, routecfg, bindingName, t); err != nil {
			return fmt.Errorf("ensuring binding %s: %w", bindingName, err)
		}
	}

	return nil
}

// ensureBinding creates or updates a RouteConfigurationBinding for the given target.
func (b *BindingCreatorBase) ensureBinding(ctx context.Context,
	routecfg *networkingv1beta1.RouteConfiguration, bindingName string, target *networkingv1beta1.RouteBindingTargetReference) error {
	binding := &networkingv1beta1.RouteConfigurationBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: routecfg.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, b.Client, binding, func() error {
		binding.Spec.RouteConfigurationRef = corev1.LocalObjectReference{Name: routecfg.Name}
		binding.Spec.TargetRef = *target
		return controllerutil.SetControllerReference(routecfg, binding, b.Scheme)
	})
	if err != nil {
		return fmt.Errorf("creating/updating binding %s/%s: %w", routecfg.Namespace, bindingName, err)
	}
	if op != controllerutil.OperationResultNone {
		klog.Infof("RouteConfigurationBinding %s/%s %s", routecfg.Namespace, bindingName, op)
	}
	return nil
}

// BindingResourceName returns the deterministic name for a RouteConfigurationBinding.
func BindingResourceName(routecfgName, entityName string) string {
	name := fmt.Sprintf("%s-%s", routecfgName, entityName)
	if len(name) <= 253 {
		return name
	}
	// entityName alone is too long to leave room for any routecfgName prefix:
	// fall back to a fully-hashed name that is always short and deterministic.
	prefixLen := 253 - 1 - len(entityName)
	if prefixLen <= 0 {
		h := fnv.New64a()
		_, _ = fmt.Fprintf(h, "%s/%s", routecfgName, entityName)
		return fmt.Sprintf("rb-%x", h.Sum64())
	}
	return fmt.Sprintf("%s-%s", routecfgName[:prefixLen], entityName)
}
