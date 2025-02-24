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
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
	// tenantClusterRolesLabelValues is the list "app.kubernetes.io/name" label values assigned to the
	// ClusterRoles to be binded to the control-plane Identity.
	tenantClusterRolesLabelValues = []string{
		"remote-controlplane",
	}

	// tenantClusterRolesClusterWideLabelValues is the list "app.kubernetes.io/name" label values assigned to the
	// ClusterRoles to be binded to the control-plane Identity with cluster-wide scope.
	tenantClusterRolesClusterWideLabelValues = []string{
		"virtual-kubelet-remote-clusterwide",
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
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants;tenants/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;deletecollection;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings/finalizers,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles/finalizers,verbs=update

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

	// If the Tenant is being deleted, we remove the finalizer and delete related resources.
	if !tenant.DeletionTimestamp.IsZero() {
		// To allow the deletion of the resource we should first remove the ClusterRoleBindings.
		if err := r.NamespaceManager.UnbindClusterRolesClusterWide(ctx, tenant.Spec.ClusterID, r.tenantClusterRolesClusterWide...); err != nil {
			klog.Errorf("Unable to unbind the ClusterRolesClusterWide for the Tenant %q before deletion: %s", req.Name, err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.enforceTenantFinalizerAbsence(ctx, tenant)
	}

	// Check if the tenant has a finalizer and if not, add it.
	if !controllerutil.ContainsFinalizer(tenant, tenantControllerFinalizer) {
		return ctrl.Result{}, r.enforceTenantFinalizerPresence(ctx, tenant)
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

	// If no handshake is tolerated, then do not perform the checks on the exchanged keys.
	if authv1beta1.GetAuthzPolicyValue(tenant.Spec.AuthzPolicy) != authv1beta1.TolerateNoHandshake {
		// get the nonce for the tenant

		nonceSecret, err := getters.GetNonceSecretByClusterID(ctx, r.Client, clusterID, corev1.NamespaceAll)
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

	// If no handshake is performed, then the user is charge of creating the authentication params and bind the right permissions.
	if authv1beta1.GetAuthzPolicyValue(tenant.Spec.AuthzPolicy) != authv1beta1.TolerateNoHandshake {
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

		_, err = r.NamespaceManager.BindClusterRoles(ctx, tenant.Spec.ClusterID, tenant, r.tenantClusterRoles...)
		if err != nil {
			klog.Errorf("Unable to bind the ClusterRoles for the Tenant %q: %s", req.Name, err)
			r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesBindingFailed", err.Error())
			return ctrl.Result{}, err
		}

		_, err = r.NamespaceManager.BindClusterRolesClusterWide(ctx, tenant.Spec.ClusterID, nil, r.tenantClusterRolesClusterWide...)
		if err != nil {
			klog.Errorf("Unable to bind the ClusterRolesClusterWide for the Tenant %q: %s", req.Name, err)
			r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesClusterWideBindingFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// getCusterRoles returns the ClusterRoles having the `app.kubernetes.io/name` equals to the provided strings.
func (r *TenantReconciler) getClusterRoles(ctx context.Context, rolesAppLabel []string) ([]*rbacv1.ClusterRole, error) {
	res := []*rbacv1.ClusterRole{}
	for _, roleName := range rolesAppLabel {
		var clusterRoles rbacv1.ClusterRoleList
		if err := r.List(ctx, &clusterRoles, client.MatchingLabels{consts.K8sAppNameKey: roleName}); err != nil {
			return nil, err
		}

		if len(clusterRoles.Items) == 0 {
			return nil, fmt.Errorf("required ClusterRole resources with %s=%q not found", consts.K8sAppNameKey, roleName)
		}

		for i := range clusterRoles.Items {
			res = append(res, &clusterRoles.Items[i])
		}
	}
	return res, nil
}

func (r *TenantReconciler) ensureSetup(ctx context.Context) error {
	if len(r.tenantClusterRoles) == 0 {
		cRoles, err := r.getClusterRoles(ctx, tenantClusterRolesLabelValues)

		if err != nil {
			return fmt.Errorf("unable to get ClusterRoles to bind on tenant namespace: %w", err)
		}

		r.tenantClusterRoles = cRoles
	}

	if len(r.tenantClusterRolesClusterWide) == 0 {
		cRoles, err := r.getClusterRoles(ctx, tenantClusterRolesClusterWideLabelValues)

		if err != nil {
			return fmt.Errorf("unable to get ClusterRoles to bind cluster-wide: %w", err)
		}

		r.tenantClusterRolesClusterWide = cRoles
	}

	return nil
}

// SetupWithManager sets up the TenantReconciler with the Manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlTenant).
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
		r.tenantClusterRolesClusterWide...); err != nil {
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesClusterWideUnbindingFailed", err.Error())
		return err
	}

	// Delete binding of cluster roles
	if err := r.NamespaceManager.UnbindClusterRoles(ctx, tenant.Spec.ClusterID, r.tenantClusterRoles...); err != nil {
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
