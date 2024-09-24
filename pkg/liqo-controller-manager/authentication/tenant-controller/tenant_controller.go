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

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
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

	tenantClusterRolesClusterWide = []string{
		"liqo-virtual-kubelet-remote-clusterwide",
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

	tenantClusterRoles            []*rbacv1.ClusterRole
	tenantClusterRolesClusterWide []*rbacv1.ClusterRole
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
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;deletecollection;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete

// Reconcile manages the lifecycle of a Tenant.
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	if err = r.ensureSetup(ctx); err != nil {
		klog.Errorf("Unable to setup the TenantReconciler: %s", err)
		return ctrl.Result{}, err
	}

	tenant := &authv1beta1.Tenant{}
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
	switch tenant.Spec.TenantCondition {
	case authv1beta1.TenantConditionDrained:
		if err := r.handleTenantDrained(ctx, tenant); err != nil {
			klog.Errorf("Unable to handle drained Tenant %q: %s", req.Name, err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	case authv1beta1.TenantConditionCordoned:
		if err := r.handleTenantCordoned(ctx, tenant); err != nil {
			klog.Errorf("Unable to handle cordoned Tenant %q: %s", req.Name, err)
			return ctrl.Result{}, err
		}
	default:
		if err := r.handleTenantUncordoned(ctx, tenant); err != nil {
			klog.Errorf("Unable to handle uncordoned Tenant %q: %s", req.Name, err)
			return ctrl.Result{}, err
		}
	}

	clusterID := tenant.Spec.ClusterID

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
		tenant.Spec.CSR, tenant.Spec.PublicKey, tenant.Spec.ClusterID); err != nil {
		klog.Errorf("Invalid CSR for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "InvalidCSR", err.Error())
		return ctrl.Result{}, nil
	}

	// create the tenant namespace

	tenantNamespace, err := r.NamespaceManager.CreateNamespace(ctx, tenant.Spec.ClusterID)
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
		Cluster:         tenant.Spec.ClusterID,
		TenantNamespace: tenant.Status.TenantNamespace,
		IdentityType:    authv1beta1.ControlPlaneIdentityType,
		Name:            tenant.Name,
		SigningRequest:  tenant.Spec.CSR,

		APIServerAddressOverride: r.APIServerAddressOverride,
		CAOverride:               r.CAOverride,
		TrustedCA:                r.TrustedCA,
		ProxyURL:                 tenant.Spec.ProxyURL,
	})
	if err != nil {
		klog.Errorf("Unable to forge the AuthParams for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "AuthParamsFailed", err.Error())
		return ctrl.Result{}, err
	}

	tenant.Status.AuthParams = authParams

	// bind permissions

	_, err = r.NamespaceManager.BindClusterRoles(ctx, tenant.Spec.ClusterID,
		tenant, r.tenantClusterRoles...)
	if err != nil {
		klog.Errorf("Unable to bind the ClusterRoles for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesBindingFailed", err.Error())
		return ctrl.Result{}, err
	}

	_, err = r.NamespaceManager.BindClusterRolesClusterWide(ctx, tenant.Spec.ClusterID,
		tenant, r.tenantClusterRolesClusterWide...)
	if err != nil {
		klog.Errorf("Unable to bind the ClusterRolesClusterWide for the Tenant %q: %s", req.Name, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesClusterWideBindingFailed", err.Error())
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

	if r.tenantClusterRolesClusterWide == nil || len(r.tenantClusterRolesClusterWide) == 0 {
		r.tenantClusterRolesClusterWide = make([]*rbacv1.ClusterRole, len(tenantClusterRolesClusterWide))
		for i, roleName := range tenantClusterRolesClusterWide {
			role := &rbacv1.ClusterRole{}
			if err := r.Get(ctx, client.ObjectKey{Name: roleName}, role); err != nil {
				return err
			}
			r.tenantClusterRolesClusterWide[i] = role
		}
	}

	return nil
}

// SetupWithManager sets up the TenantReconciler with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1beta1.Tenant{}).
		Owns(&corev1.Namespace{}).
		Complete(r)
}

func (r *TenantReconciler) handleTenantCordoned(ctx context.Context, tenant *authv1beta1.Tenant) error {
	// Cordon all the resourceslices related to the tenant
	resSlices, err := getters.ListResourceSlicesByLabel(ctx, r.Client, corev1.NamespaceAll,
		liqolabels.RemoteLabelSelectorForCluster(string(tenant.Spec.ClusterID)))
	if err != nil {
		klog.Errorf("Failed to retrieve ResourceSlices for Tenant %q: %v", tenant.Name, err)
		return err
	}

	for i := range resSlices {
		rs := &resSlices[i]
		if rs.Annotations == nil {
			rs.Annotations = make(map[string]string)
		}
		rs.Annotations[consts.CordonTenantAnnotation] = "true"

		if err := r.Client.Update(ctx, rs); err != nil {
			klog.Errorf("Failed to update ResourceSlice %q for Tenant %q: %v", client.ObjectKeyFromObject(rs), tenant.Name, err)
			return err
		}
	}

	return nil
}

func (r *TenantReconciler) handleTenantUncordoned(ctx context.Context, tenant *authv1beta1.Tenant) error {
	// Uncordon all the resourceslices related to the tenant
	resSlices, err := getters.ListResourceSlicesByLabel(ctx, r.Client, corev1.NamespaceAll,
		liqolabels.RemoteLabelSelectorForCluster(string(tenant.Spec.ClusterID)))
	if err != nil {
		klog.Errorf("Failed to retrieve ResourceSlices for Tenant %q: %v", tenant.Name, err)
		return err
	}

	for i := range resSlices {
		rs := &resSlices[i]
		if rs.Annotations == nil {
			continue
		}
		_, ok := rs.Annotations[consts.CordonTenantAnnotation]
		if !ok {
			continue
		}

		delete(rs.Annotations, consts.CordonTenantAnnotation)

		if err := r.Client.Update(ctx, rs); err != nil {
			klog.Errorf("Failed to update ResourceSlice %q for Tenant %q: %v", client.ObjectKeyFromObject(rs), tenant.Name, err)
			return err
		}
	}

	return nil
}

func (r *TenantReconciler) handleTenantDrained(ctx context.Context, tenant *authv1beta1.Tenant) error {
	// Delete binding of cluster roles cluster wide
	if err := r.NamespaceManager.UnbindClusterRolesClusterWide(ctx, tenant.Spec.ClusterID,
		tenantClusterRolesClusterWide...); err != nil {
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesClusterWideUnbindingFailed", err.Error())
		return err
	}

	// Delete binding of cluster roles
	if err := r.NamespaceManager.UnbindClusterRoles(ctx, tenant.Spec.ClusterID, tenantClusterRoles...); err != nil {
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesUnbindingFailed", err.Error())
		return err
	}
	r.EventRecorder.Event(tenant, corev1.EventTypeNormal, "ClusterRolesUnbindingSuccess", "ClusterRoles unbinded")

	// Delete all resourceslices related to the tenant
	resSlices, err := getters.ListResourceSlicesByLabel(ctx, r.Client, corev1.NamespaceAll,
		liqolabels.RemoteLabelSelectorForCluster(string(tenant.Spec.ClusterID)))
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
		liqolabels.RemoteLabelSelectorForCluster(string(tenant.Spec.ClusterID)))
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
