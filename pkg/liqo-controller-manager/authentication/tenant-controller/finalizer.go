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

package tenantcontroller

import (
	"context"

	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
)

const (
	// tenantControllerFinalizer is the finalizer added to tenant to allow the controller to clean up.
	tenantControllerFinalizer = "tenant.liqo.io/finalizer"
)

func (r *TenantReconciler) enforceTenantFinalizerPresence(ctx context.Context, tenant *authv1beta1.Tenant) error {
	ctrlutil.AddFinalizer(tenant, tenantControllerFinalizer)
	return r.Client.Update(ctx, tenant)
}

func (r *TenantReconciler) enforceTenantFinalizerAbsence(ctx context.Context, tenant *authv1beta1.Tenant) error {
	ctrlutil.RemoveFinalizer(tenant, tenantControllerFinalizer)
	return r.Client.Update(ctx, tenant)
}
