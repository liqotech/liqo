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

// CleanupPendingAttachFinalizers removes finalizers from any FirewallConfigurationAttach
// resources pending deletion that match one of the given label sets.
// It is called after the manager has fully stopped to unblock resources that the
// reconciler did not have time to process before the pod was terminated.
func CleanupPendingAttachFinalizers(ctx context.Context, cl client.Client, labelsSets []labels.Set) {
	klog.Info("Gateway stopped: cleaning up pending FirewallConfigurationAttach finalizers")
	for k := range labelsSets {
		attachList := &networkingv1beta1.FirewallConfigurationAttachList{}
		if err := cl.List(ctx, attachList, client.MatchingLabels(labelsSets[k])); err != nil {
			klog.Errorf("Shutdown cleanup: failed to list FirewallConfigurationAttach for labels %v: %v",
				labelsSets[k], err)
			continue
		}
		for i := range attachList.Items {
			klog.Infof("Shutdown cleanup: processing FirewallConfigurationAttach %s/%s", attachList.Items[i].Namespace, attachList.Items[i].Name)
			cleanupAttach(ctx, cl, &attachList.Items[i])
		}
	}
	klog.Info("Gateway stopped: completed cleanup of pending FirewallConfigurationAttach finalizers")
}

func cleanupAttach(ctx context.Context, cl client.Client, attach *networkingv1beta1.FirewallConfigurationAttach) {
	if attach.DeletionTimestamp.IsZero() {
		klog.Infof("Shutdown cleanup: FirewallConfigurationAttach %s/%s is not pending deletion, skipping\n",
			attach.Namespace, attach.Name)
		return
	}
	if !ctrlutil.ContainsFinalizer(attach, firewallConfigurationAttachControllerFinalizer) {
		klog.Infof("Shutdown cleanup: FirewallConfigurationAttach %s/%s does not have the controller finalizer, skipping\n",
			attach.Namespace, attach.Name)
		return
	}

	ctrlutil.RemoveFinalizer(attach, firewallConfigurationAttachControllerFinalizer)
	if err := cl.Update(ctx, attach); err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("Shutdown cleanup: failed to remove finalizer from FirewallConfigurationAttach %s/%s: %v",
			attach.Namespace, attach.Name, err)
		return
	}
	klog.Infof("Shutdown cleanup: removed finalizer from FirewallConfigurationAttach %s/%s",
		attach.Namespace, attach.Name)
}
