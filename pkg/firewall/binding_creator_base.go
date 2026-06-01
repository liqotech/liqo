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

package firewall

import (
	"context"
	"fmt"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// OldFirewallConfigurationFinalizer is the finalizer used by the legacy (pre-binding)
// firewall controller. It is no longer needed because cleanup is now handled per-binding,
// but it may still be present on FirewallConfiguration resources created before the
// migration. The binding creator controllers automatically strip it.
const OldFirewallConfigurationFinalizer = "firewallconfigurations-controller.liqo.io/finalizer"

// BindingTarget represents a single entity (node or gateway) that needs a FirewallConfigurationBinding.
type BindingTarget struct {
	// EntityName is the node name (for fabric) or gateway name (for gateway).
	EntityName string
	// BindingLabels are the labels to set on the FirewallConfigurationBinding resource.
	BindingLabels map[string]string
}

// BindingCreatorBase contains the shared logic for creating and managing
// FirewallConfigurationBinding resources. It is embedded by the fabric and
// gateway specific binding creator reconcilers.
type BindingCreatorBase struct {
	client.Client
	Scheme *runtime.Scheme
}

// ReconcileBindings is the core orchestration method shared by both the fabric and
// gateway binding creator reconcilers. It strips the legacy finalizer, ensures a
// binding exists for each target, and deletes stale bindings.
func (b *BindingCreatorBase) ReconcileBindings(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration, targets []BindingTarget) error {
	// Remove the legacy finalizer that was managed by the old per-FirewallConfiguration
	// controller. It is no longer needed because each binding now carries its own
	// finalizer, but clusters upgraded from the previous architecture may still have
	// it set. Stripping it here prevents the resource from being stuck during deletion.
	if controllerutil.ContainsFinalizer(fwcfg, OldFirewallConfigurationFinalizer) {
		original := fwcfg.DeepCopy()
		controllerutil.RemoveFinalizer(fwcfg, OldFirewallConfigurationFinalizer)
		if err := b.Patch(ctx, fwcfg, client.MergeFrom(original)); err != nil {
			return fmt.Errorf("removing old finalizer from %s/%s: %w", fwcfg.Namespace, fwcfg.Name, err)
		}
		klog.Infof("Removed old finalizer from FirewallConfiguration %s/%s", fwcfg.Namespace, fwcfg.Name)
	}

	// Ensure a binding exists for each target.
	expectedNames := make(map[string]struct{}, len(targets))
	for i := range targets {
		t := &targets[i]
		bindingName := BindingResourceName(fwcfg.Name, t.EntityName)
		expectedNames[bindingName] = struct{}{}
		if err := b.ensureBinding(ctx, fwcfg, bindingName, t.EntityName, t.BindingLabels); err != nil {
			return fmt.Errorf("ensuring binding %s: %w", bindingName, err)
		}
	}

	// Delete bindings that no longer have a corresponding target.
	if err := b.deleteStaleBindings(ctx, fwcfg, expectedNames); err != nil {
		return fmt.Errorf("cleaning stale bindings for %s/%s: %w", fwcfg.Namespace, fwcfg.Name, err)
	}

	return nil
}

// ensureBinding creates or updates a FirewallConfigurationBinding for the given entity.
func (b *BindingCreatorBase) ensureBinding(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration, bindingName, targetID string, labels map[string]string) error {
	binding := &networkingv1beta1.FirewallConfigurationBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: fwcfg.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, b.Client, binding, func() error {
		binding.Labels = labels
		binding.Spec.FirewallConfigurationRef = corev1.LocalObjectReference{Name: fwcfg.Name}
		binding.Spec.TargetID = targetID
		return controllerutil.SetControllerReference(fwcfg, binding, b.Scheme)
	})
	if err != nil {
		return fmt.Errorf("creating/updating binding %s/%s: %w", fwcfg.Namespace, bindingName, err)
	}
	if op != controllerutil.OperationResultNone {
		klog.V(4).Infof("FirewallConfigurationBinding %s/%s %s", fwcfg.Namespace, bindingName, op)
	}
	return nil
}

// deleteStaleBindings removes FirewallConfigurationBinding resources owned by fwcfg
// whose names are not in expectedNames.
func (b *BindingCreatorBase) deleteStaleBindings(ctx context.Context,
	fwcfg *networkingv1beta1.FirewallConfiguration, expectedNames map[string]struct{}) error {
	bindingList := &networkingv1beta1.FirewallConfigurationBindingList{}
	if err := b.List(ctx, bindingList, client.InNamespace(fwcfg.Namespace)); err != nil {
		return fmt.Errorf("listing firewallconfigurationbindings: %w", err)
	}

	for i := range bindingList.Items {
		binding := &bindingList.Items[i]
		if !IsOwnedBy(binding, fwcfg.UID) {
			continue
		}
		if _, ok := expectedNames[binding.Name]; ok {
			continue
		}
		klog.V(4).Infof("Deleting stale FirewallConfigurationBinding %s/%s", binding.Namespace, binding.Name)
		// Remove the gateway-pod finalizer before deleting so the resource is not stuck if the
		// gateway pod is already gone (the pod cannot remove its own finalizer when it is dead).
		if controllerutil.ContainsFinalizer(binding, FirewallConfigurationBindingControllerFinalizer) {
			original := binding.DeepCopy()
			controllerutil.RemoveFinalizer(binding, FirewallConfigurationBindingControllerFinalizer)
			if err := b.Patch(ctx, binding, client.MergeFrom(original)); err != nil {
				if !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
					return fmt.Errorf("removing finalizer from stale binding %s/%s: %w", binding.Namespace, binding.Name, err)
				}
			}
		}
		if err := b.Delete(ctx, binding); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting binding %s/%s: %w", binding.Namespace, binding.Name, err)
		}
	}
	return nil
}

// EnqueueFirewallConfigurationsByCategory lists all FirewallConfiguration resources with the
// given category label and returns reconcile requests for each.
func (b *BindingCreatorBase) EnqueueFirewallConfigurationsByCategory(
	ctx context.Context, category string) []reconcile.Request {
	fwcfgList := &networkingv1beta1.FirewallConfigurationList{}
	if err := b.List(ctx, fwcfgList, client.MatchingLabels{
		FirewallCategoryTargetKey: category,
	}); err != nil {
		klog.Errorf("Unable to list FirewallConfigurations with category %q: %v", category, err)
		return nil
	}

	requests := make([]reconcile.Request, 0, len(fwcfgList.Items))
	for i := range fwcfgList.Items {
		item := &fwcfgList.Items[i]
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: item.Name, Namespace: item.Namespace},
		})
	}
	return requests
}

// IsOwnedBy reports whether obj has an ownerReference with the given UID.
func IsOwnedBy(obj metav1.Object, ownerUID types.UID) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == ownerUID {
			return true
		}
	}
	return false
}

// BindingResourceName returns the deterministic name for a FirewallConfigurationBinding.
func BindingResourceName(fwcfgName, entityName string) string {
	name := fmt.Sprintf("%s-%s", fwcfgName, entityName)
	if len(name) <= 253 {
		return name
	}
	// entityName alone is too long to leave room for any fwcfgName prefix:
	// fall back to a fully-hashed name that is always short and deterministic.
	prefixLen := 253 - 1 - len(entityName)
	if prefixLen <= 0 {
		h := fnv.New64a()
		_, _ = fmt.Fprintf(h, "%s/%s", fwcfgName, entityName)
		return fmt.Sprintf("fwb-%x", h.Sum64())
	}
	return fmt.Sprintf("%s-%s", fwcfgName[:prefixLen], entityName)
}
