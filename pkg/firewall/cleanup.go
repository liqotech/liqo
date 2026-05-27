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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

// CleanupPendingBindingFinalizers removes finalizers from any FirewallConfigurationBinding
// resources pending deletion that match one of the given label sets.
// It is called after the manager has fully stopped to unblock resources that the
// reconciler did not have time to process before the pod was terminated.
func CleanupPendingBindingFinalizers(ctx context.Context, cl client.Client, labelsSets []labels.Set) {
	klog.Info("Gateway stopped: cleaning up pending FirewallConfigurationBinding finalizers")
	for k := range labelsSets {
		bindingList := &networkingv1beta1.FirewallConfigurationBindingList{}
		if err := cl.List(ctx, bindingList, client.MatchingLabels(labelsSets[k])); err != nil {
			klog.Errorf("Shutdown cleanup: failed to list FirewallConfigurationBinding for labels %v: %v",
				labelsSets[k], err)
			continue
		}
		for i := range bindingList.Items {
			klog.Infof("Shutdown cleanup: processing FirewallConfigurationBinding %s/%s", bindingList.Items[i].Namespace, bindingList.Items[i].Name)
			cleanupBinding(ctx, cl, &bindingList.Items[i])
		}
	}
	klog.Info("Gateway stopped: completed cleanup of pending FirewallConfigurationBinding finalizers")
}

func cleanupBinding(ctx context.Context, cl client.Client, binding *networkingv1beta1.FirewallConfigurationBinding) {
	if binding.DeletionTimestamp.IsZero() {
		klog.Infof("Shutdown cleanup: FirewallConfigurationBinding %s/%s is not pending deletion, skipping\n",
			binding.Namespace, binding.Name)
		return
	}
	if !ctrlutil.ContainsFinalizer(binding, firewallConfigurationBindingControllerFinalizer) {
		klog.Infof("Shutdown cleanup: FirewallConfigurationBinding %s/%s does not have the controller finalizer, skipping\n",
			binding.Namespace, binding.Name)
		return
	}

	ctrlutil.RemoveFinalizer(binding, firewallConfigurationBindingControllerFinalizer)
	if err := cl.Update(ctx, binding); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Shutdown cleanup: failed to remove finalizer from FirewallConfigurationBinding %s/%s: %v",
			binding.Namespace, binding.Name, err)
		return
	}
	klog.Infof("Shutdown cleanup: removed finalizer from FirewallConfigurationBinding %s/%s",
		binding.Namespace, binding.Name)
}
