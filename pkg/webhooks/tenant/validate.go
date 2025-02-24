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
	"errors"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
)

// cluster-role
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants,verbs=get;list;watch;
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;

type tenantValidatorWebhook struct {
	client client.Client
	tenantDecoder
}

// NewValidator returns a new Tenant validating webhook.
func NewValidator(cl client.Client) *webhook.Admission {
	return &webhook.Admission{
		Handler: &tenantValidatorWebhook{
			tenantDecoder: tenantDecoder{
				decoder: admission.NewDecoder(runtime.NewScheme()),
			},
			client: cl,
		},
	}
}

// Handle implements the Tenant validating webhook logic.
//
//nolint:gocritic // The signature of this method is imposed by controller runtime.
func (w *tenantValidatorWebhook) Handle(ctx context.Context, req admission.Request) admission.Response {
	switch req.Operation {
	case admissionv1.Create:
		return w.handleCreate(ctx, &req)
	case admissionv1.Update:
		return w.handleUpdate(ctx, &req)
	default:
		return admission.Allowed("")
	}
}

func (w *tenantValidatorWebhook) handleCreate(ctx context.Context, req *admission.Request) admission.Response {
	tenant, err := w.DecodeTenant(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding Tenant object: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if status, err := w.tenantConsistencyChecks(ctx, tenant); err != nil {
		return admission.Errored(status, err)
	}

	// Check that there is one single Tenant in the tenant namespace.
	tenantsInNamespace, err := w.getTenants(ctx, tenant.Namespace, nil)
	if err != nil {
		werr := fmt.Errorf("failed getting Tenants in tenant namespace: %v", output.PrettyErr(err))
		klog.Error(werr)
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed getting Tenants in tenant namespace"))
	}

	if len(tenantsInNamespace) > 0 {
		return admission.Denied("a Tenant already exists in the tenant namespace")
	}

	// Check that the Tenant name is unique in the entire cluster.
	tenantsInCluster, err := w.getTenants(ctx, corev1.NamespaceAll, &tenant.Name)
	if err != nil {
		werr := fmt.Errorf("failed getting Tenants in cluster: %v", output.PrettyErr(err))
		klog.Error(werr)
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed getting Tenants in cluster"))
	}

	if len(tenantsInCluster) > 0 {
		return admission.Denied("tenant should have a unique name across the cluster: a Tenant with the same name already exists in the cluster")
	}

	return admission.Allowed("")
}

func (w *tenantValidatorWebhook) handleUpdate(ctx context.Context, req *admission.Request) admission.Response {
	tenant, err := w.DecodeTenant(req.Object)
	if err != nil {
		klog.Errorf("Failed decoding Tenant object: %v", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if status, err := w.tenantConsistencyChecks(ctx, tenant); err != nil {
		return admission.Errored(status, err)
	}

	// Check that the Tenant name is unique in the entire cluster.
	tenantsInCluster, err := w.getTenants(ctx, corev1.NamespaceAll, &tenant.Name)
	if err != nil {
		werr := fmt.Errorf("failed getting Tenants in cluster: %v", output.PrettyErr(err))
		klog.Error(werr)
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed getting Tenants in cluster"))
	}

	// If there already is a Tenant with the given name but it is not the same Tenant, deny the update.
	if len(tenantsInCluster) == 1 && tenantsInCluster[0].UID != tenant.UID {
		return admission.Denied("tenant should have a unique name across the cluster: a Tenant with the same name already exists in the cluster")
	}

	return admission.Allowed("")
}

func (w *tenantValidatorWebhook) tenantConsistencyChecks(ctx context.Context, tenant *authv1beta1.Tenant) (code int32, err error) {
	// Check that the Tenant has been created in the proper tenant namespace.
	var ns corev1.Namespace
	if err := w.client.Get(ctx, client.ObjectKey{Name: tenant.Namespace}, &ns); err != nil {
		werr := fmt.Errorf("failed getting the tenant namespace: %v", output.PrettyErr(err))
		return http.StatusInternalServerError, werr
	}

	if ns.Labels == nil || ns.Labels[consts.TenantNamespaceLabel] != "true" {
		return http.StatusBadRequest, errors.New("resources Tenant must be created in a tenant namespace, check the documentation for further info")
	}

	// Check that the liqo.io/remote-cluster-id label matches the one in the specs.
	if tenant.Labels[consts.RemoteClusterID] != string(tenant.Spec.ClusterID) {
		return http.StatusBadRequest, fmt.Errorf("the %q label must match the cluster ID in the specs", consts.RemoteClusterID)
	}

	// Check that the Tenant has the same cluster ID of its tenant namespace.
	if string(tenant.Spec.ClusterID) != ns.Labels[consts.RemoteClusterID] {
		return http.StatusForbidden, errors.New("the Tenant must have the same cluster ID of its tenant namespace")
	}

	return http.StatusOK, nil
}

func (w *tenantValidatorWebhook) getTenants(ctx context.Context, namespace string, name *string) ([]authv1beta1.Tenant, error) {
	var tenantList authv1beta1.TenantList

	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	if name != nil {
		opts = append(opts, client.MatchingFields{"metadata.name": *name})
	}

	if err := w.client.List(ctx, &tenantList, opts...); err != nil {
		return nil, err
	}

	return tenantList.Items, nil
}
