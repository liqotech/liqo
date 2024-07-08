// Copyright 2019-2024 The Liqo Authors
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

package util

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
)

// ActivateTenants sets the TenantCondition of all Tenants to Active.
func ActivateTenants(ctx context.Context, cl client.Client) error {
	return modifyTenants(ctx, cl, func(tenant *authv1alpha1.Tenant) {
		tenant.Spec.TenantCondition = authv1alpha1.TenantConditionActive
	})
}

// CordonTenants sets the TenantCondition of all Tenants to Cordoned.
func CordonTenants(ctx context.Context, cl client.Client) error {
	return modifyTenants(ctx, cl, func(tenant *authv1alpha1.Tenant) {
		tenant.Spec.TenantCondition = authv1alpha1.TenantConditionCordoned
	})
}

// DrainTenants sets the TenantCondition of all Tenants to Drained.
func DrainTenants(ctx context.Context, cl client.Client) error {
	return modifyTenants(ctx, cl, func(tenant *authv1alpha1.Tenant) {
		tenant.Spec.TenantCondition = authv1alpha1.TenantConditionDrained
	})
}

func modifyTenants(ctx context.Context, cl client.Client, mutation func(*authv1alpha1.Tenant)) error {
	var tenants authv1alpha1.TenantList
	if err := cl.List(ctx, &tenants); err != nil {
		return err
	}

	for i := range tenants.Items {
		tenant := tenants.Items[i]
		mutation(&tenant)
		if err := cl.Update(ctx, &tenant); err != nil {
			return err
		}
	}

	return nil
}
