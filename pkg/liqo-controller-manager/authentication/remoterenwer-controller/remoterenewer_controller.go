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

package remoterenwercontroller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/events"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// RemoteRenewerReconciler reconciles an Renew object.
type RemoteRenewerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	NamespaceManager         tenantnamespace.Manager
	IdentityProvider         identitymanager.IdentityProvider
	APIServerAddressOverride string
	CAOverride               []byte
	TrustedCA                bool
	recorder                 record.EventRecorder
}

// NewRemoteRenewerReconciler returns a new RemoteRenewerReconciler.
func NewRemoteRenewerReconciler(cl client.Client, s *runtime.Scheme,
	identityProvider identitymanager.IdentityProvider,
	namespaceManager tenantnamespace.Manager,
	apiServerAddressOverride string, caOverride []byte, trustedCA bool,
	recorder record.EventRecorder) *RemoteRenewerReconciler {
	return &RemoteRenewerReconciler{
		Client: cl,
		Scheme: s,

		NamespaceManager:         namespaceManager,
		IdentityProvider:         identityProvider,
		APIServerAddressOverride: apiServerAddressOverride,
		CAOverride:               caOverride,
		TrustedCA:                trustedCA,
		recorder:                 recorder,
	}
}

//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews,verbs=get;list;watch;update;patch;delete;create
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=,resources=secrets,verbs=get;list;watch;update;patch

// Reconcile implements the logic to reconcile an Renew object.
//
// The function first retrieves the Renew object and the related tenant and namespace.
// If the namespace of the Renew object doesn't match with the tenant namespace, it skips the reconciliation.
// Then, it calls the handleRenew function to generate the certificate for the renew and update the Renew object.
// Finally, if the renew is for a control plane or a resource slice, it calls the updateTenantStatusOnRenew or
// updateResourceSliceStatusOnRenew function to update the tenant or resource slice status.
//
// If an error occurs during the process, the function logs the error and returns it.
func (r *RemoteRenewerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var renew authv1beta1.Renew
	if err := r.Get(ctx, req.NamespacedName, &renew); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	tenantNamespace, err := r.NamespaceManager.GetNamespace(ctx, renew.Spec.ConsumerClusterID)
	if err != nil {
		klog.Errorf("Unable to get tenant namespace for Renew %q: %s", req.NamespacedName, err)
		events.EventWithOptions(r.recorder, &renew, fmt.Sprintf("Failed to get tenant namespace: %s", err),
			&events.Option{EventType: events.Error, Reason: "TenantNamespaceNotFound"})
		return ctrl.Result{}, err
	}

	if tenantNamespace.Name != renew.Namespace {
		klog.V(4).Infof("Skipping Renew %q as it's not in the tenant namespace %q", req.NamespacedName, tenantNamespace.Name)
		events.EventWithOptions(r.recorder, &renew, fmt.Sprintf("Skipping renewal as it's not in tenant namespace %s", tenantNamespace.Name),
			&events.Option{EventType: events.Warning, Reason: "WrongNamespace"})
		return ctrl.Result{}, nil
	}

	tenant, err := getters.GetTenantByClusterID(ctx, r.Client, renew.Spec.ConsumerClusterID, tenantNamespace.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	var resourceSlice *authv1beta1.ResourceSlice
	if renew.Spec.ResourceSliceRef != nil {
		resourceSlice = &authv1beta1.ResourceSlice{}
		if err := r.Get(ctx, client.ObjectKey{
			Namespace: tenantNamespace.Name,
			Name:      renew.Spec.ResourceSliceRef.Name,
		}, resourceSlice); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.handleRenew(ctx, &renew, tenant, resourceSlice); err != nil {
		klog.Errorf("Unable to handle Renew %q: %s", req.NamespacedName, err)
		events.EventWithOptions(r.recorder, &renew, fmt.Sprintf("Failed to handle renewal: %s", err),
			&events.Option{EventType: events.Error, Reason: "RenewalFailed"})
		return ctrl.Result{}, err
	}

	klog.V(4).Infof("Successfully handled Renew %q", req.NamespacedName)
	events.Event(r.recorder, &renew, "Successfully renewed certificate")

	switch {
	case renew.Spec.IdentityType == authv1beta1.ControlPlaneIdentityType:
		err = r.updateTenantStatusOnRenew(ctx, &renew, tenant)
		if err != nil {
			return ctrl.Result{}, err
		}
	case renew.Spec.IdentityType == authv1beta1.ResourceSliceIdentityType && resourceSlice != nil:
		err = r.updateResourceSliceStatusOnRenew(ctx, &renew, resourceSlice)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RemoteRenewerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p, err := predicate.LabelSelectorPredicate(reflection.ReplicatedResourcesLabelSelector())
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlRenewRemote).
		For(&authv1beta1.Renew{}, builder.WithPredicates(p)).
		Complete(r)
}

// updateResourceSliceStatusOnRenew updates the status of the given ResourceSlice with the
// authParams obtained by the given Renew.
//
// The function updates the ResourceSlice object in the cluster with the obtained authParams.
// If the update fails, the function logs an error and returns the error.
//
// Args:
//   - ctx: the context of the request
//   - renew: the Renew object containing the authParams
//   - resourceSlice: the ResourceSlice object to be updated
//
// Returns:
//   - error: the error occurred during the update, if any
func (r *RemoteRenewerReconciler) updateResourceSliceStatusOnRenew(ctx context.Context,
	renew *authv1beta1.Renew,
	resourceSlice *authv1beta1.ResourceSlice) error {
	resourceSlice.Status.AuthParams = renew.Status.AuthParams
	if err := r.Status().Update(ctx, resourceSlice); err != nil {
		klog.Errorf("Failed to update ResourceSlice status for %q: %s", resourceSlice.Name, err)
		return err
	}

	return nil
}

// updateTenantStatusOnRenew updates the status of the given Tenant with the
// authParams obtained by the given Renew.
//
// The function updates the Tenant object in the cluster with the obtained authParams.
// If the update fails, the function logs an error and returns the error.
//
// Args:
//   - ctx: the context of the request
//   - renew: the Renew object containing the authParams
//   - tenant: the Tenant object to be updated
//
// Returns:
//   - error: the error occurred during the update, if any
func (r *RemoteRenewerReconciler) updateTenantStatusOnRenew(ctx context.Context,
	renew *authv1beta1.Renew,
	tenant *authv1beta1.Tenant) error {
	tenant.Status.AuthParams = renew.Status.AuthParams
	if err := r.Status().Update(ctx, tenant); err != nil {
		klog.Errorf("Failed to update Tenant status for %q: %s", tenant.Name, err)
		return err
	}

	return nil
}

// handleRenew handles a Renew object, creating the corresponding AuthParams and updating the Renew status.
//
// Args:
//   - ctx: the context of the request
//   - renew: the Renew object to be handled
//   - tenant: the Tenant object associated with the Renew
//   - resourceSlice: the ResourceSlice object associated with the Renew, if any
//
// Returns:
//   - error: the error occurred during the handling, if any
func (r *RemoteRenewerReconciler) handleRenew(ctx context.Context,
	renew *authv1beta1.Renew,
	tenant *authv1beta1.Tenant,
	resourceSlice *authv1beta1.ResourceSlice) error {
	var name string
	if resourceSlice != nil {
		name = resourceSlice.Name
	} else {
		name = tenant.Name
	}

	authParams, err := r.IdentityProvider.ForgeAuthParams(ctx, &identitymanager.SigningRequestOptions{
		Cluster:         renew.Spec.ConsumerClusterID,
		TenantNamespace: tenant.Status.TenantNamespace,
		IdentityType:    renew.Spec.IdentityType,
		Name:            name,
		SigningRequest:  renew.Spec.CSR,

		APIServerAddressOverride: r.APIServerAddressOverride,
		CAOverride:               r.CAOverride,
		TrustedCA:                r.TrustedCA,
		ResourceSlice:            resourceSlice,
		ProxyURL:                 tenant.Spec.ProxyURL,
		IsUpdate:                 true,
	})
	if err != nil {
		klog.Errorf("Unable to forge the AuthParams for the Renew %q: %s", renew.Name, err)
		return err
	}

	renew.Status.AuthParams = authParams
	if err := r.Status().Update(ctx, renew); err != nil {
		klog.Errorf("Failed to update Renew status for %q: %s", renew.Name, err)
		return err
	}

	return nil
}
