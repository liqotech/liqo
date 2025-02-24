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

package remoteresourceslicecontroller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// NewRemoteResourceSliceReconciler returns a new RemoteResourceSliceReconciler.
func NewRemoteResourceSliceReconciler(cl client.Client, s *runtime.Scheme, config *rest.Config,
	recorder record.EventRecorder,
	identityProvider identitymanager.IdentityProvider,
	namespaceManager tenantnamespace.Manager,
	apiServerAddressOverride string, caOverride []byte, trustedCA bool,
	sliceStatusOptions *SliceStatusOptions) *RemoteResourceSliceReconciler {
	return &RemoteResourceSliceReconciler{
		Client: cl,
		Scheme: s,
		Config: config,

		eventRecorder:    recorder,
		identityProvider: identityProvider,
		namespaceManager: namespaceManager,

		apiServerAddressOverride: apiServerAddressOverride,
		caOverride:               caOverride,
		trustedCA:                trustedCA,

		sliceStatusOptions: sliceStatusOptions,

		reconciledClasses: []authv1beta1.ResourceSliceClass{
			authv1beta1.ResourceSliceClassDefault,
			authv1beta1.ResourceSliceClassUnknown,
		},
	}
}

// RemoteResourceSliceReconciler reconciles a ResourceSliceReconciler object.
type RemoteResourceSliceReconciler struct {
	client.Client
	*runtime.Scheme
	Config *rest.Config

	eventRecorder    record.EventRecorder
	identityProvider identitymanager.IdentityProvider
	namespaceManager tenantnamespace.Manager

	apiServerAddressOverride string
	caOverride               []byte
	trustedCA                bool

	sliceStatusOptions *SliceStatusOptions

	reconciledClasses []authv1beta1.ResourceSliceClass
}

// cluster-role
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices;resourceslices/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage,resources=storageclasses,verbs=get;list;watch

// Reconcile replicated ResourceSlice resources.
func (r *RemoteResourceSliceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	var resourceSlice authv1beta1.ResourceSlice
	if err = r.Get(ctx, req.NamespacedName, &resourceSlice); err != nil {
		if kerrors.IsNotFound(err) {
			klog.V(4).Infof("resourceSlice %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("unable to get ResourceSlice %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if resourceSlice.Spec.ConsumerClusterID == nil {
		err = fmt.Errorf("ConsumerClusterID not set")
		klog.Errorf("Unable to ensure the remote certificate for the ResourceSlice %q: %s", req.NamespacedName, err)
		r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "RemoteCertificateFailed", err.Error())
		return ctrl.Result{}, nil
	}

	tenantNamespace, err := r.namespaceManager.GetNamespace(ctx, *resourceSlice.Spec.ConsumerClusterID)
	if err != nil {
		return ctrl.Result{}, err
	}

	if tenantNamespace.Name != resourceSlice.Namespace {
		klog.Errorf("The namespace %q of the ResourceSlice %q doesn't match with the tenant namespace %q, skipping",
			resourceSlice.Namespace, resourceSlice.Name, tenantNamespace.Name)
		return ctrl.Result{}, nil
	}

	// Get Tenant associated with the ResourceSlice.
	tenant, err := getters.GetTenantByClusterID(ctx, r.Client, *resourceSlice.Spec.ConsumerClusterID, tenantNamespace.Name)
	if err != nil {
		klog.Errorf("Unable to get the Tenant for the ResourceSlice %q: %s", req.NamespacedName, err)
		r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "TenantNotFound", err.Error())
		return ctrl.Result{}, err
	}

	// Defer the update of the ResourceSlice status.
	defer func() {
		errDef := r.Client.Status().Update(ctx, &resourceSlice)
		if errDef != nil {
			klog.Errorf("Unable to update the ResourceSlice %q: %s", req.NamespacedName, errDef)
			r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "UpdateFailed", errDef.Error())
			err = errDef
		}

		if err == nil {
			r.eventRecorder.Event(&resourceSlice, corev1.EventTypeNormal, "Reconciled", "ResourceSlice reconciled")
		}
	}()

	// Make sure that the resource slice has been created in the tentant namespace dedicated to the current consumer
	err = validateRSNamespace(ctx, r.Client, req.Namespace, string(*resourceSlice.Spec.ConsumerClusterID))
	switch {
	case kerrors.IsBadRequest(err):
		klog.Errorf("Invalid ResourceSlice %q provided: %s", req.NamespacedName, err)
		r.eventRecorder.Event(&resourceSlice, corev1.EventTypeWarning, "Invalid ResourceSlice", err.Error())
		denyAuthentication(&resourceSlice, r.eventRecorder)
		return ctrl.Result{}, nil
	case err != nil:
		klog.Errorf("unable to get tenant Namespace of ResourceSlice %q: %v", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	// Handle the ResourceSlice authentication status
	if err = r.handleAuthenticationStatus(ctx, &resourceSlice, tenant); err != nil {
		return ctrl.Result{}, err
	}

	// Handle the ResourceSlice resources status
	if err = r.handleResourcesStatus(ctx, &resourceSlice, tenant); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RemoteResourceSliceReconciler) handleAuthenticationStatus(ctx context.Context,
	resourceSlice *authv1beta1.ResourceSlice, tenant *authv1beta1.Tenant) error {
	// check that the CSR is valid
	shouldCheckPublicKey := authv1beta1.GetAuthzPolicyValue(tenant.Spec.AuthzPolicy) != authv1beta1.TolerateNoHandshake
	if err := authentication.CheckCSRForResourceSlice(tenant.Spec.PublicKey, resourceSlice, shouldCheckPublicKey); err != nil {
		klog.Errorf("Invalid CSR for the ResourceSlice %q: %s", client.ObjectKeyFromObject(resourceSlice), err)
		r.eventRecorder.Event(resourceSlice, corev1.EventTypeWarning, "InvalidCSR", err.Error())
		denyAuthentication(resourceSlice, r.eventRecorder)
		return nil
	}

	// forge the AuthParams
	authParams, err := r.identityProvider.ForgeAuthParams(ctx, &identitymanager.SigningRequestOptions{
		Cluster:         *resourceSlice.Spec.ConsumerClusterID,
		TenantNamespace: resourceSlice.Namespace,
		IdentityType:    authv1beta1.ResourceSliceIdentityType,
		Name:            resourceSlice.Name,
		SigningRequest:  resourceSlice.Spec.CSR,

		APIServerAddressOverride: r.apiServerAddressOverride,
		CAOverride:               r.caOverride,
		TrustedCA:                r.trustedCA,
		ResourceSlice:            resourceSlice,
		ProxyURL:                 tenant.Spec.ProxyURL,
	})
	if err != nil {
		klog.Errorf("Unable to forge the AuthParams for the ResourceSlice %q: %s", client.ObjectKeyFromObject(resourceSlice), err)
		r.eventRecorder.Event(resourceSlice, corev1.EventTypeWarning, "AuthParamsFailed", err.Error())
		denyAuthentication(resourceSlice, r.eventRecorder)
		return err
	}

	resourceSlice.Status.AuthParams = authParams

	// accept the authentication
	acceptAuthentication(resourceSlice, r.eventRecorder)

	return nil
}

func (r *RemoteResourceSliceReconciler) handleResourcesStatus(ctx context.Context,
	resourceSlice *authv1beta1.ResourceSlice, tenant *authv1beta1.Tenant) error {
	var err error

	switch tenant.Spec.TenantCondition {
	case authv1beta1.TenantConditionActive:
		// If the ResourceSlice is not of the default class, the resource status is leaved as it is and the update is
		// demanded to external controllers/plugins.
		if !isInResourceClasses(resourceSlice, r.reconciledClasses...) {
			klog.V(6).Infof("ResourceSlice %q is not of the default class, the resource status is leaved as it is",
				client.ObjectKeyFromObject(resourceSlice))
			return nil
		}

		// Default class: accept requested resources and set the default values for the resources not specified.
		findOrDefault := func(resource corev1.ResourceName, val resource.Quantity) resource.Quantity {
			v, ok := resourceSlice.Spec.Resources[resource]
			if ok {
				return v
			}
			return val
		}

		if resourceSlice.Status.Resources == nil {
			resourceSlice.Status.Resources = corev1.ResourceList{}
		}

		for k, v := range r.sliceStatusOptions.DefaultResourceQuantity {
			resourceSlice.Status.Resources[k] = findOrDefault(k, v)
		}
		for k, v := range resourceSlice.Spec.Resources {
			resourceSlice.Status.Resources[k] = v
		}

		resourceSlice.Status.StorageClasses, err = getStorageClasses(ctx, r.Client, r.sliceStatusOptions)
		if err != nil {
			klog.Errorf("Unable to get the StorageClasses for the ResourceSlice %q: %s", client.ObjectKeyFromObject(resourceSlice), err)
			r.eventRecorder.Event(resourceSlice, corev1.EventTypeWarning, "StorageClassesFailed", err.Error())
			return err
		}

		resourceSlice.Status.IngressClasses = getIngressClasses(r.sliceStatusOptions)
		resourceSlice.Status.LoadBalancerClasses = getLoadBalancerClasses(r.sliceStatusOptions)
		resourceSlice.Status.NodeLabels = getNodeLabels(r.sliceStatusOptions)

		acceptResources(resourceSlice, r.eventRecorder)
	case authv1beta1.TenantConditionCordoned:
		// Only deny if the resources are not already accepted.
		resCond := authentication.GetCondition(resourceSlice, authv1beta1.ResourceSliceConditionTypeResources)
		if resCond == nil || resCond.Status == "" {
			denyResources(resourceSlice, r.eventRecorder)
		}
	case authv1beta1.TenantConditionDrained:
		denyResources(resourceSlice, r.eventRecorder)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RemoteResourceSliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// generate the predicate to filter just the ResourceSlices created by the remote cluster checking crdReplicator labels
	remoteResSliceFilter, err := predicate.LabelSelectorPredicate(reflection.ReplicatedResourcesLabelSelector())
	if err != nil {
		klog.Error(err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlResourceSliceRemote).
		For(
			&authv1beta1.ResourceSlice{},
			// With GenerationChangedPredicate we prevent to reconcile multiple times when the status of the resource changes
			builder.WithPredicates(predicate.And(remoteResSliceFilter, withCSR(), predicate.GenerationChangedPredicate{})),
		).
		Watches(&authv1beta1.Tenant{}, handler.EnqueueRequestsFromMapFunc(r.resourceSlicesEnquer())).
		Complete(r)
}

func (r *RemoteResourceSliceReconciler) resourceSlicesEnquer() func(ctx context.Context, obj client.Object) []reconcile.Request {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		tenant, ok := obj.(*authv1beta1.Tenant)
		if !ok {
			klog.Infof("Object %q is not a Tenant", obj.GetName())
			return nil
		}

		if tenant.Spec.ClusterID == "" {
			klog.Infof("ClusterID not set for Tenant %q", tenant.Name)
			return nil
		}
		resSlices, err := getters.ListResourceSlicesByLabel(ctx, r.Client, corev1.NamespaceAll,
			liqolabels.RemoteLabelSelectorForCluster(string(tenant.Spec.ClusterID)))
		if err != nil {
			klog.Errorf("Failed to retrieve ResourceSlices for Tenant %q: %v", tenant.Name, err)
			return nil
		}

		reqs := make([]reconcile.Request, len(resSlices))
		for i := range resSlices {
			reqs[i] = reconcile.Request{NamespacedName: types.NamespacedName{
				Name:      resSlices[i].Name,
				Namespace: resSlices[i].Namespace,
			}}
		}

		return reqs
	}
}

func withCSR() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		rs, ok := obj.(*authv1beta1.ResourceSlice)
		if !ok {
			return false
		}
		return len(rs.Spec.CSR) > 0
	})
}

func acceptAuthentication(resourceSlice *authv1beta1.ResourceSlice, er record.EventRecorder) {
	switch authentication.EnsureCondition(
		resourceSlice,
		authv1beta1.ResourceSliceConditionTypeAuthentication,
		authv1beta1.ResourceSliceConditionAccepted,
		"ResourceSliceAuthenticationAccepted",
		"ResourceSlice authentication accepted",
	) {
	case controllerutil.OperationResultNone:
		klog.V(4).Infof("ResourceSlice authentication %q already accepted", resourceSlice.Name)
	case controllerutil.OperationResultUpdated, controllerutil.OperationResultCreated:
		klog.Infof("ResourceSlice authentication %q accepted", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceAuthenticationAccepted", "ResourceSlice authentication accepted")
	default:
		return
	}
}

func denyAuthentication(resourceSlice *authv1beta1.ResourceSlice, er record.EventRecorder) {
	switch authentication.EnsureCondition(
		resourceSlice,
		authv1beta1.ResourceSliceConditionTypeAuthentication,
		authv1beta1.ResourceSliceConditionDenied,
		"ResourceSliceAuthenticationDenied",
		"ResourceSlice authentication denied",
	) {
	case controllerutil.OperationResultNone:
		klog.V(4).Infof("ResourceSlice authentication %q already denied", resourceSlice.Name)
	case controllerutil.OperationResultUpdated, controllerutil.OperationResultCreated:
		klog.Infof("ResourceSlice authentication %q denied", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceAuthenticationDenied", "ResourceSlice authentication denied")
	default:
		return
	}
}

func acceptResources(resourceSlice *authv1beta1.ResourceSlice, er record.EventRecorder) {
	switch authentication.EnsureCondition(
		resourceSlice,
		authv1beta1.ResourceSliceConditionTypeResources,
		authv1beta1.ResourceSliceConditionAccepted,
		"ResourceSliceResourcesAccepted",
		"ResourceSlice resources accepted",
	) {
	case controllerutil.OperationResultNone:
		klog.V(4).Infof("ResourceSlice resources %q already accepted", resourceSlice.Name)
	case controllerutil.OperationResultUpdated:
		klog.Infof("ResourceSlice resources %q accepted", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceResourcesAccepted", "ResourceSlice resources updated")
	case controllerutil.OperationResultCreated:
		klog.Infof("ResourceSlice resources %q accepted", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceResourcesAccepted", "ResourceSlice resources accepted")
	default:
		return
	}
}

func denyResources(resourceSlice *authv1beta1.ResourceSlice, er record.EventRecorder) {
	switch authentication.EnsureCondition(
		resourceSlice,
		authv1beta1.ResourceSliceConditionTypeResources,
		authv1beta1.ResourceSliceConditionDenied,
		"ResourceSliceResourcesDenied",
		"ResourceSlice resources denied",
	) {
	case controllerutil.OperationResultNone:
		klog.V(4).Infof("ResourceSlice resources %q already denied", resourceSlice.Name)
	case controllerutil.OperationResultUpdated:
		klog.Infof("ResourceSlice resources %q denied", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceResourcesDenied", "ResourceSlice resources updated")
	case controllerutil.OperationResultCreated:
		klog.Infof("ResourceSlice resources %q denied", resourceSlice.Name)
		er.Event(resourceSlice, corev1.EventTypeNormal, "ResourceSliceResourcesDenied", "ResourceSlice resources denied")
	default:
		return
	}
}

func isInResourceClasses(resourceSlice *authv1beta1.ResourceSlice, classes ...authv1beta1.ResourceSliceClass) bool {
	for _, class := range classes {
		if resourceSlice.Spec.Class == class {
			return true
		}
	}
	return false
}

// validateRSNamespace makes sure that the ResourceSlice has been created in the tenant namespace dedicated to the consumer cluster.
func validateRSNamespace(ctx context.Context, c client.Client, namespace, consumerClusterID string) error {
	var tenantNamespace corev1.Namespace
	if err := c.Get(ctx, types.NamespacedName{Name: namespace}, &tenantNamespace); err != nil {
		return err
	}

	if tenantLabel, labelPresent := tenantNamespace.Labels[consts.TenantNamespaceLabel]; !labelPresent || strings.EqualFold(tenantLabel, "false") {
		return kerrors.NewBadRequest("A ResourceSlice cannot be created in a namespace different than the Tenant namespace")
	}

	if clusterIDLabel, labelPresent := tenantNamespace.Labels[consts.RemoteClusterID]; !labelPresent || clusterIDLabel != consumerClusterID {
		return kerrors.NewBadRequest(
			fmt.Sprintf("ResourceSlice belonging to %q has been created in the Tenant namespace of %q", consumerClusterID, clusterIDLabel),
		)
	}

	return nil
}
