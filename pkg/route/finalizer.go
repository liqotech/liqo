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

package route

import (
	"context"

	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

const (
	// routeconfigurationControllerFinalizer is the finalizer added to the RouteConfiguration.
	routeconfigurationControllerFinalizer = "routeconfiguration-controller.liqo.io/finalizer"
)

func (r *RouteConfigurationReconciler) ensureRouteConfigurationFinalizerPresence(
	ctx context.Context, fwcfg *networkingv1beta1.RouteConfiguration) error {
	ctrlutil.AddFinalizer(fwcfg, routeconfigurationControllerFinalizer)
	return r.Client.Update(ctx, fwcfg)
}

func (r *RouteConfigurationReconciler) ensureRouteConfigurationFinalizerAbsence(
	ctx context.Context, fwcfg *networkingv1beta1.RouteConfiguration) error {
	ctrlutil.RemoveFinalizer(fwcfg, routeconfigurationControllerFinalizer)
	return r.Client.Update(ctx, fwcfg)
}
