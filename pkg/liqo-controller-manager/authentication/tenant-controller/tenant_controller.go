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
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
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
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	responsetypes "github.com/liqotech/liqo/pkg/identityManager/responseTypes"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	noncecreatorcontroller "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/noncecreator-controller"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

var (
	tenantClusterRoles = []string{
		"liqo-control-plane",
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
			klog.Infof("Tenant %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the Tenant %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	clusterID := tenant.Spec.ClusterIdentity.ClusterID

	// get the nonce for the tenant

	nonceSecret, err := getters.GetNonceByClusterID(ctx, r.Client, clusterID)
	if err != nil {
		klog.Errorf("Unable to get the nonce for the Tenant %q: %s", req.NamespacedName, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "NonceNotFound", err.Error())
		return ctrl.Result{}, err
	}

	nonce, err := noncecreatorcontroller.GetNonceFromSecret(nonceSecret)
	if err != nil {
		klog.Errorf("Unable to get the nonce for the Tenant %q: %s", req.NamespacedName, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "NonceNotFound", err.Error())
		return ctrl.Result{}, err
	}

	// check the signature

	if !authentication.VerifyNonce(ed25519.PublicKey(tenant.Spec.PublicKey), nonce, tenant.Spec.Signature) {
		err = fmt.Errorf("signature verification failed for Tenant %q", req.NamespacedName)
		klog.Error(err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "SignatureVerificationFailed", err.Error())
		return ctrl.Result{}, nil
	}

	// check that the CSR is created with the same public key

	if err = checkCSR(tenant.Spec.CSR, tenant.Spec.PublicKey, &tenant.Spec.ClusterIdentity); err != nil {
		klog.Errorf("Invalid CSR for the Tenant %q: %s", req.NamespacedName, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "InvalidCSR", err.Error())
		return ctrl.Result{}, nil
	}

	// create the tenant namespace

	tenantNamespace, err := r.NamespaceManager.CreateNamespace(ctx, tenant.Spec.ClusterIdentity)
	if err != nil {
		klog.Errorf("Unable to create the TenantNamespace for the Tenant %q: %s", req.NamespacedName, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "TenantNamespaceCreationFailed", err.Error())
		return ctrl.Result{}, err
	}

	tenant.Status.TenantNamespace = tenantNamespace.Name

	defer func() {
		errDef := r.Client.Status().Update(ctx, tenant)
		if errDef != nil {
			klog.Errorf("Unable to update the Tenant %q: %s", req.NamespacedName, errDef)
			r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "UpdateFailed", errDef.Error())
			err = errDef
			return
		}

		r.EventRecorder.Event(tenant, corev1.EventTypeNormal, "Reconciled", "Tenant reconciled")
	}()

	// create the CSR and forge the AuthParams

	resp, err := r.IdentityProvider.GetRemoteCertificate(tenant.Spec.ClusterIdentity, tenant.Status.TenantNamespace, tenant.Spec.CSR)
	switch {
	case apierrors.IsNotFound(err) && resp.ResponseType == responsetypes.SigningRequestResponseIAM:
		// iam always returns not found, so we can ignore it
	case apierrors.IsNotFound(err):
		resp, err = r.IdentityProvider.ApproveSigningRequest(tenant.Spec.ClusterIdentity, tenant.Spec.CSR)
		if err != nil {
			klog.Errorf("Unable to approve the CSR for the Tenant %q: %s", req.NamespacedName, err)
			r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "CSRApprovalFailed", err.Error())
			return ctrl.Result{}, err
		}
	case err != nil:
		klog.Errorf("Unable to get the remote certificate for the Tenant %q: %s", req.NamespacedName, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "RemoteCertificateFailed", err.Error())
		return ctrl.Result{}, err
	}

	apiServer, err := apiserver.GetURL(ctx, r.APIServerAddressOverride, r.Client)
	if err != nil {
		klog.Errorf("Unable to get the API server URL: %s", err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "APIServerURLFailed", err.Error())
		return ctrl.Result{}, err
	}

	ca, err := apiserver.RetrieveAPIServerCA(r.Config, r.CAOverride, r.TrustedCA)
	if err != nil {
		klog.Errorf("Unable to get the API server CA: %s", err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "APIServerCAFailed", err.Error())
		return ctrl.Result{}, err
	}

	switch resp.ResponseType {
	case responsetypes.SigningRequestResponseCertificate:
		tenant.Status.AuthParams = forgeAuthParamsCert(resp, apiServer, ca)
	case responsetypes.SigningRequestResponseIAM:
		tenant.Status.AuthParams = forgeAuthParamsIAM(resp, apiServer, ca)
	default:
		err = fmt.Errorf("unexpected response type %q", resp.ResponseType)
		klog.Error(err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "UnexpectedResponseType", err.Error())
		return ctrl.Result{}, err
	}

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
		klog.Errorf("Unable to bind the ClusterRoles for the Tenant %q: %s", req.NamespacedName, err)
		r.EventRecorder.Event(tenant, corev1.EventTypeWarning, "ClusterRolesBindingFailed", err.Error())
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func checkCSR(csr, publicKey []byte, remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity) error {
	pemCsr, rst := pem.Decode(csr)
	if pemCsr == nil || len(rst) != 0 {
		return fmt.Errorf("invalid CSR")
	}

	x509Csr, err := x509.ParseCertificateRequest(pemCsr.Bytes)
	if err != nil {
		return err
	}

	if x509Csr.Subject.CommonName != authentication.CommonName(*remoteClusterIdentity) {
		return fmt.Errorf("invalid common name")
	}

	// if the pub key is 0-terminated, drop it
	if publicKey[len(publicKey)-1] == 0 {
		publicKey = publicKey[:len(publicKey)-1]
	}

	switch crtKey := x509Csr.PublicKey.(type) {
	case ed25519.PublicKey:
		if !bytes.Equal(crtKey, publicKey) {
			return fmt.Errorf("invalid public key")
		}
	default:
		return fmt.Errorf("invalid public key type %T", crtKey)
	}

	return nil
}

func forgeAuthParamsCert(resp *responsetypes.SigningRequestResponse, apiServer string, ca []byte) *authv1alpha1.AuthParams {
	return &authv1alpha1.AuthParams{
		CA:        ca,
		SignedCRT: resp.Certificate,
		APIServer: apiServer,
	}
}

func forgeAuthParamsIAM(resp *responsetypes.SigningRequestResponse, apiServer string, ca []byte) *authv1alpha1.AuthParams {
	return &authv1alpha1.AuthParams{
		CA:        ca,
		APIServer: apiServer,
		AwsConfig: &authv1alpha1.AwsConfig{
			AwsAccessKeyID:     *resp.AwsIdentityResponse.AccessKey.AccessKeyId,
			AwsSecretAccessKey: *resp.AwsIdentityResponse.AccessKey.SecretAccessKey,
			AwsRegion:          resp.AwsIdentityResponse.Region,
			AwsClusterName:     *resp.AwsIdentityResponse.EksCluster.Name,
		},
	}
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
