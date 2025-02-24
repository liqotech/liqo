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

package tenant

import (
	"context"
	"encoding/json"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/liqotech/liqo/pkg/consts"
)

type tenantMutatorWebhook struct {
	client client.Client
	tenantDecoder
}

// NewMutator returns a new Tenant mutating webhook.
func NewMutator(cl client.Client) *webhook.Admission {
	return &webhook.Admission{
		Handler: &tenantMutatorWebhook{
			tenantDecoder: tenantDecoder{
				decoder: admission.NewDecoder(runtime.NewScheme()),
			},
			client: cl,
		},
	}
}

// Handle implements the tenant mutate webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *tenantMutatorWebhook) Handle(_ context.Context, req admission.Request) admission.Response {
	tenant, err := w.DecodeTenant(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding Tenant object: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Add the label remote-cluster-id label if not provided if not provided
	if tenant.Labels == nil {
		tenant.Labels = map[string]string{}
	}

	if _, ok := tenant.Labels[consts.RemoteClusterID]; ok {
		return admission.Allowed("")
	}

	tenant.Labels[consts.RemoteClusterID] = string(tenant.Spec.ClusterID)

	marshaledTenant, err := json.Marshal(tenant)
	if err != nil {
		klog.Errorf("Failed encoding tenant in admission response: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledTenant)
}
