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

	"github.com/google/nftables"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	firewallapi "github.com/liqotech/liqo/apis/networking/v1beta1/firewall"
)

// CleanupFirewallConfigurationBindings removes finalizers from any FirewallConfigurationBinding
// resources pending deletion whose spec.targetID matches the given targetID.
// If cleanupNftables is true, it also deletes the corresponding nftables table for each binding,
// following the same approach used by the FirewallConfigurationBinding controller deletion path.
// It is called after the manager has fully stopped to unblock resources that the
// reconciler did not have time to process before the pod was terminated.
func CleanupFirewallConfigurationBindings(ctx context.Context, cl client.Client, targetID string, cleanupNftables bool) {
	klog.Info("Gateway stopped: cleaning up pending FirewallConfigurationBinding finalizers")

	var nftconn *nftables.Conn
	if cleanupNftables {
		var err error
		nftconn, err = nftables.New()
		if err != nil {
			klog.Errorf("Shutdown cleanup: failed to create nftables connection: %v", err)
		}
	}

	bindingList := &networkingv1beta1.FirewallConfigurationBindingList{}
	if err := cl.List(ctx, bindingList); err != nil {
		klog.Errorf("Shutdown cleanup: failed to list FirewallConfigurationBinding resources: %v", err)
		return
	}
	for i := range bindingList.Items {
		if bindingList.Items[i].Spec.TargetID == targetID {
			klog.Infof("Shutdown cleanup: processing FirewallConfigurationBinding %s/%s", bindingList.Items[i].Namespace, bindingList.Items[i].Name)
			cleanupBinding(ctx, cl, &bindingList.Items[i], nftconn)
		}
	}
	klog.Info("Gateway stopped: completed cleanup of pending FirewallConfigurationBinding finalizers")
}

func cleanupBinding(ctx context.Context, cl client.Client, binding *networkingv1beta1.FirewallConfigurationBinding, nftconn *nftables.Conn) {
	if binding.DeletionTimestamp.IsZero() {
		klog.Infof("Shutdown cleanup: FirewallConfigurationBinding %s/%s is not pending deletion, skipping",
			binding.Namespace, binding.Name)
		return
	}
	if !ctrlutil.ContainsFinalizer(binding, firewallConfigurationBindingControllerFinalizer) {
		klog.Infof("Shutdown cleanup: FirewallConfigurationBinding %s/%s does not have the controller finalizer, skipping\n",
			binding.Namespace, binding.Name)
		return
	}

	if nftconn != nil && binding.Status.TableName != "" {
		tableName := binding.Status.TableName
		delTable(nftconn, &firewallapi.Table{Name: &tableName})
		if err := nftconn.Flush(); err != nil {
			klog.Errorf("Shutdown cleanup: failed to flush nftables for FirewallConfigurationBinding %s/%s: %v",
				binding.Namespace, binding.Name, err)
		} else {
			klog.Infof("Shutdown cleanup: deleted nftables table %q for FirewallConfigurationBinding %s/%s",
				tableName, binding.Namespace, binding.Name)
		}
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
