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

	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const (
	firewallConfigurationAttachControllerFinalizer = "firewallconfigurationattach-controller.liqo.io/finalizer"
	// FirewallConfigurationAttachControllerFinalizer is the exported alias used by external
	// packages (e.g. liqo-controller-manager) to reference the same finalizer string.
	FirewallConfigurationAttachControllerFinalizer = firewallConfigurationAttachControllerFinalizer
)

func (r *FirewallConfigurationAttachReconciler) ensureAttachFinalizerPresence(
	ctx context.Context, fwattach *networkingv1beta1.FirewallConfigurationAttach) error {
	ctrlutil.AddFinalizer(fwattach, firewallConfigurationAttachControllerFinalizer)
	return r.Client.Update(ctx, fwattach)
}

func (r *FirewallConfigurationAttachReconciler) ensureAttachFinalizerAbsence(
	ctx context.Context, fwattach *networkingv1beta1.FirewallConfigurationAttach) error {
	ctrlutil.RemoveFinalizer(fwattach, firewallConfigurationAttachControllerFinalizer)
	return r.Client.Update(ctx, fwattach)
}
