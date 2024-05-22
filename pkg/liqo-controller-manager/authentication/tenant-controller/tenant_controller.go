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

package tenantcontroller

import (
	"context"
	"crypto/ed25519"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	authgetters "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/getters"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

var (
	tenantClusterRoles = []string{
		"liqo-remote-controlplane",
	}
)

// TenantReconciler manages the lifecycle of a Tenant.
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config

	EventRecorder    record.EventRecorder
	IdentityProvider identitymanager.IdentityProvider
	NamespaceManager tenantnamespace.Manager

	APIServerAddressOverride string
	CAOverride               []byte
	TrustedCA                bool

	tenantClusterRoles []*rbacv1.ClusterRole
}

// NewTenantReconciler creates a new TenantReconciler.
func NewTenantReconciler(cl client.Client, scheme *runtime.Scheme, config *rest.Config,
	eventRecorder record.EventRecorder, identityProvider identitymanager.IdentityProvider,
	namespaceManager tenantnamespace.Manager,
	apiServerAddressOverride string, caOverride []byte, trustedCA bool) *TenantReconciler {
	return &TenantReconciler{
		Client: cl,
		Scheme: scheme,
		Config: config,

		EventRecorder:    eventRecorder,
		IdentityProvider: identityProvider,
		NamespaceManager: namespaceManager,

		APIServerAddressOverride: apiServerAddressOverride,
		CAOverride:               caOverride,
		TrustedCA:                trustedCA,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants;tenants/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete

// Reconcile manages the lifecycle of a Tenant.
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	if err = r.ensureSetup(ctx); err != nil {
		klog.Errorf("Unable to setup the TenantReconciler: %s", err)
		return ctrl.Result{}, err
	}

	tenant := &authv1alpha1.Tenant{}
	if err = r.Get(ctx, req.NamespacedName, tenant); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Tenant %q not found", req.Name)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the Tenant %q: %s", req.Name, err)
		return ctrl.Result{}, err
	}

	// If the Tenant is drained we remove the binding of cluster roles used to replicate resources and
	// delete all replicated resources.
	if tenant.Spec.TenantCondition == authv1alpha1.TenantConditionDrained {
		if err := r.handleTenantDrained(ctx, tenant); err != nil {
			klog.Errorf("Unable to handle drained Tenant %q: %s", req.Name, err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	clusterID := tenant.Spec.ClusterIdentity.ClusterID

	// get the nonce for the tenant

	nonceSecret, err := getters.GetNonceSecretByClusterID(ctx, r.Client, clusterID)
	if err != nil {
		klog.Errorf("Unable to get the nonce for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "NonceNotFound", err.Error())
		return ctrl.Result{}, err
	}

	nonce, err := authgetters.GetNonceFromSecret(nonceSecret)
	if err != nil {
		klog.Errorf("Unable to get the nonce for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "NonceNotFound", err.Error())
		return ctrl.Result{}, err
	}

	// check the signature

	if !authentication.VerifyNonce(ed25519.PublicKey(tenant.Spec.PublicKey), nonce, tenant.Spec.Signature) {
		err = fmt.Errorf("signature verification failed for Tenant %q", req.Name)
		klog.Error(err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "SignatureVerificationFailed", err.Error())
		return ctrl.Result{}, nil
	}

	// check that the CSR is created with the same public key

	if err = authentication.CheckCSRForControlPlane(
		tenant.Spec.CSR, tenant.Spec.PublicKey, &tenant.Spec.ClusterIdentity); err != nil {
		klog.Errorf("Invalid CSR for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "InvalidCSR", err.Error())
		return ctrl.Result{}, nil
	}

	// create the tenant namespace

	tenantNamespace, err := r.NamespaceManager.CreateNamespace(ctx, tenant.Spec.ClusterIdentity)
	if err != nil {
		klog.Errorf("Unable to create the TenantNamespace for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "TenantNamespaceCreationFailed", err.Error())
		return ctrl.Result{}, err
	}

	tenant.Status.TenantNamespace = tenantNamespace.Name

	defer func() {
		errDef := r.Client.Status().Update(ctx, tenant)
		if errDef != nil {
			klog.Errorf("Unable to update the Tenant %q: %s", req.Name, errDef)
			r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "UpdateFailed", errDef.Error())
			err = errDef
		}

		if err == nil {
			r.EventRecorder.Event(tenant, corev1.EventTypeNormal, "Reconciled", "Tenant reconciled")
		}
	}()

	// create the CSR and forge the AuthParams

	authParams, err := r.IdentityProvider.ForgeAuthParams(ctx, &identitymanager.SigningRequestOptions{
		Cluster:         &tenant.Spec.ClusterIdentity,
		TenantNamespace: tenant.Status.TenantNamespace,
		IdentityType:    authv1alpha1.ControlPlaneIdentityType,
		Name:            tenant.Name,
		SigningRequest:  tenant.Spec.CSR,
	})
	if err != nil {
		klog.Errorf("Unable to forge the AuthParams for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "AuthParamsFailed", err.Error())
		return ctrl.Result{}, err
	}

	tenant.Status.AuthParams = authParams

	// own the tenant namespace

	err = controllerutil.SetOwnerReference(tenant, tenantNamespace, r.Scheme)
	if err != nil {
		klog.Errorf("Unable to set the OwnerReference for the TenantNamespace %q: %s", tenantNamespace.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "OwnerReferenceFailed", err.Error())
		return ctrl.Result{}, err
	}

	if err = r.Client.Update(ctx, tenantNamespace); err != nil {
		klog.Errorf("Unable to set the OwnerReference for the TenantNamespace %q: %s", tenantNamespace.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "OwnerReferenceFailed", err.Error())
		return ctrl.Result{}, err
	}

	// bind permissions

	_, err = r.NamespaceManager.BindClusterRoles(ctx, tenant.Spec.ClusterIdentity, r.tenantClusterRoles...)
	if err != nil {
		klog.Errorf("Unable to bind the ClusterRoles for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesBindingFailed", err.Error())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *TenantReconciler) ensureSetup(ctx context.Context) error {
	if r.tenantClusterRoles == nil || len(r.tenantClusterRoles) == 0 {
		r.tenantClusterRoles = make([]*rbacv1.ClusterRole, len(tenantClusterRoles))
		for i, roleName := range tenantClusterRoles {
			role := &rbacv1.ClusterRole{}
			if err := r.Get(ctx, client.ObjectKey{Name: roleName}, role); err != nil {
				return err
			}
			r.tenantClusterRoles[i] = role
		}
	}
	return nil
}

// SetupWithManager sets up the TenantReconciler with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1alpha1.Tenant{}).
		Owns(&corev1.Namespace{}).
		Complete(r)
}

func (r *TenantReconciler) handleTenantDrained(ctx context.Context, tenant *authv1alpha1.Tenant) error {
	// Delete binding of cluster roles
	if err := r.NamespaceManager.UnbindClusterRoles(ctx, tenant.Spec.ClusterIdentity, tenantClusterRoles...); err != nil {
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesUnbindingFailed", err.Error())
		return err
	}
	r.EventRecorder.Event(tenant, corev1.EventTypeNormal, "ClusterRolesUnbindingSuccess", "ClusterRoles unbinded")

	// Delete all resourceslices related to the tenant
	resSlices, err := getters.ListResourceSlicesByLabel(ctx, r.Client, corev1.NamespaceAll,
		liqolabels.RemoteLabelSelectorForCluster(tenant.Spec.ClusterIdentity.ClusterID))
	if err != nil {
		klog.Errorf("Failed to retrieve ResourceSlices for Tenant %q: %v", tenant.Name, err)
		return err
	}

	for i := range resSlices {
		if err := client.IgnoreNotFound(r.Client.Delete(ctx, &resSlices[i])); err != nil {
			klog.Errorf("Failed to delete ResourceSlice %q for Tenant %q: %v", client.ObjectKeyFromObject(&resSlices[i]), tenant.Name, err)
			return err
		}
	}
	r.EventRecorder.Event(tenant, corev1.EventTypeNormal, "ResourceSlicesDeleted", "ResourceSlices deleted")

	// Delete all the namespacemaps related to the tenant
	namespaceMaps, err := getters.ListNamespaceMapsByLabel(ctx, r.Client, corev1.NamespaceAll,
		liqolabels.RemoteLabelSelectorForCluster(tenant.Spec.ClusterIdentity.ClusterID))
	if err != nil {
		klog.Errorf("Failed to retrieve NamespaceMaps for Tenant %q: %v", tenant.Name, err)
		return err
	}

	for i := range namespaceMaps {
		if err := client.IgnoreNotFound(r.Client.Delete(ctx, &namespaceMaps[i])); err != nil {
			klog.Errorf("Failed to delete NamespaceMap %q for Tenant %q: %v", client.ObjectKeyFromObject(&namespaceMaps[i]), tenant.Name, err)
			return err
		}
	}
	r.EventRecorder.Event(tenant, corev1.EventTypeNormal, "NamespaceMapsDeleted", "NamespaceMaps deleted")

	return nil
}
